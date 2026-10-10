// Package scan is a read-only security scan of the agent configuration in a
// project: MCP server definitions, Claude Code hooks and project settings. It
// never writes files and never executes anything it finds.
package scan

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// Severity orders findings. The zero value is SeverityInfo.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityLow
	SeverityMedium
	SeverityHigh
)

var severityNames = []string{"info", "low", "medium", "high"}

func (s Severity) String() string {
	if s < 0 || int(s) >= len(severityNames) {
		return fmt.Sprintf("severity(%d)", int(s))
	}
	return severityNames[s]
}

// MarshalJSON encodes the severity as its lowercase name.
func (s Severity) MarshalJSON() ([]byte, error) { return json.Marshal(s.String()) }

// UnmarshalJSON accepts the lowercase name produced by MarshalJSON.
func (s *Severity) UnmarshalJSON(b []byte) error {
	var name string
	if err := json.Unmarshal(b, &name); err != nil {
		return err
	}
	v, ok := ParseSeverity(name)
	if !ok {
		return fmt.Errorf("unknown severity %q", name)
	}
	*s = v
	return nil
}

// ParseSeverity maps a case-insensitive name to a Severity.
func ParseSeverity(name string) (Severity, bool) {
	for i, n := range severityNames {
		if strings.EqualFold(name, n) {
			return Severity(i), true
		}
	}
	return 0, false
}

// Finding is one rule hit. File is relative to the scanned project directory.
// Line is 1-based and 0 when it could not be located cheaply. Message never
// contains a full secret value.
type Finding struct {
	ID       string   `json:"id"`
	Severity Severity `json:"severity"`
	File     string   `json:"file"`
	Line     int      `json:"line,omitempty"`
	Subject  string   `json:"subject"`
	Message  string   `json:"message"`

	// locator is literal text from the source used to find Line; it is kept
	// out of the serialized finding because it may be a secret.
	locator string
}

// Skipped records a config file that exists but could not be parsed.
type Skipped struct {
	File   string `json:"file"`
	Reason string `json:"reason"`
}

// Report is the result of Scan. Findings are sorted by severity (highest
// first), then file, line and rule ID, so output is stable across runs.
type Report struct {
	Scanned  []string  `json:"scanned"`
	Skipped  []Skipped `json:"skipped,omitempty"`
	Findings []Finding `json:"findings"`
}

// mcpServer is an MCP server definition normalized across harness formats.
type mcpServer struct {
	file    string
	name    string
	command string
	args    []string
	env     map[string]string
	headers map[string]string
	url     string
	// lines maps literal string values to the 1-based line they were read
	// from, for formats where that is known exactly (TOML).
	lines map[string]int
}

func (s mcpServer) commandLine() string {
	return strings.TrimSpace(s.command + " " + strings.Join(s.args, " "))
}

// hookCommand is one shell command a harness runs on an event.
type hookCommand struct {
	file    string
	event   string
	command string
}

// projectSettings is the subset of a project-scoped Claude Code settings file
// the rules inspect.
type projectSettings struct {
	file        string
	defaultMode string
	env         map[string]string
}

type inputs struct {
	servers  []mcpServer
	hooks    []hookCommand
	settings []projectSettings
}

// Scan reads every agent config file the given adapters know about under
// projectDir and runs all rules over them. Missing files are not an error.
func Scan(projectDir string, adapters []domain.Adapter) (Report, error) {
	var (
		in      inputs
		report  Report
		seen    = map[string]bool{}
		sources = map[string][]byte{}
	)

	read := func(path string) ([]byte, bool, error) {
		if seen[path] {
			return nil, false, nil
		}
		seen[path] = true
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, fmt.Errorf("read %s: %w", path, err)
		}
		rel := relPath(projectDir, path)
		report.Scanned = append(report.Scanned, rel)
		sources[rel] = data
		return data, true, nil
	}
	skip := func(path string, err error) {
		report.Skipped = append(report.Skipped, Skipped{File: relPath(projectDir, path), Reason: err.Error()})
	}

	for _, a := range adapters {
		if a.ID() == domain.AgentClaudeCode {
			path := a.ProjectConfigFile(projectDir)
			data, ok, err := read(path)
			if err != nil {
				return Report{}, err
			}
			if ok {
				hooks, settings, err := parseClaudeSettings(relPath(projectDir, path), data)
				if err != nil {
					skip(path, err)
				} else {
					in.hooks = append(in.hooks, hooks...)
					in.settings = append(in.settings, settings)
				}
			}
		}

		if a.ID() == domain.AgentCodex {
			// Codex has no project MCP path on the adapter interface; the
			// project-scoped TOML is the same file verify already reads.
			path := filepath.Join(projectDir, ".codex", "config.toml")
			data, ok, err := read(path)
			if err != nil {
				return Report{}, err
			}
			if ok {
				in.servers = append(in.servers, parseCodexTOML(relPath(projectDir, path), data)...)
			}
			continue
		}

		path := a.MCPConfigPath(projectDir)
		if path == "" {
			path = a.ProjectConfigFile(projectDir)
		}
		if path == "" || filepath.Ext(path) != ".json" {
			continue
		}
		data, ok, err := read(path)
		if err != nil {
			return Report{}, err
		}
		if !ok {
			continue
		}
		servers, err := parseMCPJSON(relPath(projectDir, path), data, a.MCPRootKey(), a.MCPEnvKey(), a.MCPURLKey())
		if err != nil {
			skip(path, err)
			continue
		}
		in.servers = append(in.servers, servers...)
	}

	report.Findings = runRules(in)
	for i := range report.Findings {
		f := &report.Findings[i]
		if f.Line == 0 {
			f.Line = locate(sources[f.File], f)
		}
	}
	sortFindings(report.Findings)
	if report.Findings == nil {
		report.Findings = []Finding{}
	}
	return report, nil
}

func runRules(in inputs) []Finding {
	var out []Finding
	for _, r := range rules {
		out = append(out, r.check(in)...)
	}
	return out
}

func sortFindings(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if a.Severity != b.Severity {
			return a.Severity > b.Severity
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.ID < b.ID
	})
}

func relPath(projectDir, path string) string {
	if rel, err := filepath.Rel(projectDir, path); err == nil {
		return filepath.ToSlash(rel)
	}
	return path
}

// locate finds the first line containing the finding's needle. It is a
// heuristic for JSON sources, where the decoder does not keep positions.
func locate(src []byte, f *Finding) int {
	needle := f.locator
	if len(src) == 0 || needle == "" {
		return 0
	}
	sc := bufio.NewScanner(strings.NewReader(string(src)))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		if strings.Contains(sc.Text(), needle) {
			return n
		}
	}
	return 0
}

func parseClaudeSettings(file string, data []byte) ([]hookCommand, projectSettings, error) {
	var doc struct {
		Permissions struct {
			DefaultMode string `json:"defaultMode"`
		} `json:"permissions"`
		Env   map[string]any `json:"env"`
		Hooks map[string][]struct {
			Hooks []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, projectSettings{}, fmt.Errorf("parse JSON: %w", err)
	}
	settings := projectSettings{file: file, defaultMode: doc.Permissions.DefaultMode, env: stringMap(doc.Env)}

	events := make([]string, 0, len(doc.Hooks))
	for e := range doc.Hooks {
		events = append(events, e)
	}
	sort.Strings(events)
	var hooks []hookCommand
	for _, e := range events {
		for _, group := range doc.Hooks[e] {
			for _, h := range group.Hooks {
				if h.Command != "" {
					hooks = append(hooks, hookCommand{file: file, event: e, command: h.Command})
				}
			}
		}
	}
	return hooks, settings, nil
}

// parseMCPJSON reads the servers under rootKey. command may be a string with a
// separate args array, or a single array (OpenCode).
func parseMCPJSON(file string, data []byte, rootKey, envKey, urlKey string) ([]mcpServer, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	raw, ok := doc[rootKey]
	if !ok {
		return nil, nil
	}
	var defs map[string]map[string]any
	if err := json.Unmarshal(raw, &defs); err != nil {
		return nil, fmt.Errorf("parse %q: %w", rootKey, err)
	}

	names := make([]string, 0, len(defs))
	for n := range defs {
		names = append(names, n)
	}
	sort.Strings(names)

	servers := make([]mcpServer, 0, len(names))
	for _, name := range names {
		def := defs[name]
		s := mcpServer{file: file, name: name}
		switch c := def["command"].(type) {
		case string:
			s.command = c
		case []any:
			parts := stringSlice(c)
			if len(parts) > 0 {
				s.command, s.args = parts[0], parts[1:]
			}
		}
		if args, ok := def["args"].([]any); ok {
			s.args = append(s.args, stringSlice(args)...)
		}
		s.env = stringMap(asMap(def[envKey]))
		if envKey != "env" && s.env == nil {
			s.env = stringMap(asMap(def["env"]))
		}
		s.headers = stringMap(asMap(def["headers"]))
		for _, k := range []string{urlKey, "url"} {
			if u, ok := def[k].(string); ok && u != "" {
				s.url = u
				break
			}
		}
		servers = append(servers, s)
	}
	return servers, nil
}

// parseCodexTOML line-scans [mcp_servers.<name>] tables. It is not a TOML
// parser: it understands one-line string values, one-line string arrays and
// one-line inline tables, plus [mcp_servers.<name>.env] subtables. Anything
// else is ignored rather than guessed at.
func parseCodexTOML(file string, data []byte) []mcpServer {
	var (
		servers []mcpServer
		cur     *mcpServer
		inEnv   bool
	)
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			header := strings.Trim(line, "[] ")
			cur, inEnv = nil, false
			rest, ok := strings.CutPrefix(header, "mcp_servers.")
			if !ok {
				continue
			}
			name := strings.Trim(rest, `"`)
			if base, sub, found := strings.Cut(rest, "."); found {
				name = strings.Trim(base, `"`)
				inEnv = sub == "env"
				if !inEnv {
					continue
				}
			}
			idx := -1
			for i := range servers {
				if servers[i].name == name {
					idx = i
				}
			}
			if idx < 0 {
				servers = append(servers, mcpServer{file: file, name: name, env: map[string]string{}, lines: map[string]int{}})
				idx = len(servers) - 1
			}
			cur = &servers[idx]
			continue
		}
		if cur == nil {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.Trim(strings.TrimSpace(key), `"`)
		val = strings.TrimSpace(val)
		strs := tomlStrings(val)
		for _, s := range strs {
			if _, dup := cur.lines[s]; !dup {
				cur.lines[s] = n
			}
		}
		if inEnv {
			if len(strs) == 1 {
				cur.env[key] = strs[0]
			}
			continue
		}
		switch key {
		case "command":
			if len(strs) == 1 {
				cur.command = strs[0]
			}
		case "args":
			cur.args = strs
		case "url":
			if len(strs) == 1 {
				cur.url = strs[0]
			}
		case "env", "http_headers":
			pairs := tomlInlineTable(val)
			target := cur.env
			if key == "http_headers" {
				if cur.headers == nil {
					cur.headers = map[string]string{}
				}
				target = cur.headers
			}
			for k, v := range pairs {
				target[k] = v
			}
		}
	}
	return servers
}

// tomlStrings returns every basic or literal string on a TOML value line.
func tomlStrings(val string) []string {
	var out []string
	for i := 0; i < len(val); i++ {
		q := val[i]
		if q != '"' && q != '\'' {
			continue
		}
		var b strings.Builder
		j := i + 1
		for ; j < len(val) && val[j] != q; j++ {
			if q == '"' && val[j] == '\\' && j+1 < len(val) {
				j++
			}
			b.WriteByte(val[j])
		}
		out = append(out, b.String())
		i = j
	}
	return out
}

// tomlInlineTable parses `{ KEY = "v", OTHER = "w" }` on one line.
func tomlInlineTable(val string) map[string]string {
	out := map[string]string{}
	val = strings.TrimSpace(val)
	if !strings.HasPrefix(val, "{") || !strings.HasSuffix(val, "}") {
		return out
	}
	for _, part := range splitOutsideQuotes(val[1:len(val)-1], ',') {
		k, v, ok := strings.Cut(part, "=")
		if !ok {
			continue
		}
		if strs := tomlStrings(v); len(strs) == 1 {
			out[strings.Trim(strings.TrimSpace(k), `"`)] = strs[0]
		}
	}
	return out
}

func splitOutsideQuotes(s string, sep byte) []string {
	var (
		parts []string
		start int
		quote byte
	)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == '\\' && quote == '"' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == sep:
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	return append(parts, s[start:])
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func stringMap(m map[string]any) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

func stringSlice(in []any) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
