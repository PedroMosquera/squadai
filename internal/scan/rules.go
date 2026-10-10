package scan

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
)

// Rule describes one scan check for help text and docs.
type Rule struct {
	ID       string   `json:"id"`
	Severity Severity `json:"severity"`
	Title    string   `json:"title"`
}

type rule struct {
	Rule
	check func(inputs) []Finding
}

var rules = []rule{
	{Rule{"SCAN-001", SeverityHigh, "secret literal in MCP server, hook or settings env"}, checkSecrets},
	{Rule{"SCAN-002", SeverityHigh, "remote script fetched and executed at runtime"}, checkRemoteExec},
	{Rule{"SCAN-003", SeverityMedium, "package runner without a pinned version"}, checkUnpinnedRunner},
	{Rule{"SCAN-004", SeverityHigh, "hook command executes or splits tool input"}, checkHookInjection},
	{Rule{"SCAN-005", SeverityLow, "project setting Claude Code ignores"}, checkIgnoredProjectSettings},
	{Rule{"SCAN-006", SeverityMedium, "MCP server over plain http to a remote host"}, checkPlainHTTP},
}

// Rules returns the metadata of every rule, in ID order. Severity is the
// highest a rule reports; SCAN-004 reports medium for unquoted expansion.
func Rules() []Rule {
	out := make([]Rule, len(rules))
	for i, r := range rules {
		out[i] = r.Rule
	}
	return out
}

func serverFinding(id string, sev Severity, s mcpServer, locator, msg string) Finding {
	return Finding{ID: id, Severity: sev, File: s.file, Line: s.lines[locator], Subject: fmt.Sprintf("mcp server %q", s.name), Message: msg, locator: locator}
}

func hookFinding(id string, sev Severity, h hookCommand, locator, msg string) Finding {
	return Finding{ID: id, Severity: sev, File: h.file, Subject: "hook " + h.event, Message: msg, locator: locator}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

var tokenPatterns = []struct {
	kind string
	re   *regexp.Regexp
}{
	{"GitHub token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{36,}`)},
	{"GitHub fine-grained token", regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{22,}`)},
	{"Slack token", regexp.MustCompile(`\bxox[abposr]-[A-Za-z0-9-]{10,}`)},
	{"AWS access key ID", regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"API key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}`)},
}

var assignmentRe = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_-]*)=(?:"([^"]*)"|'([^']*)'|([^\s;&|"']+))`)

func secretKeyName(key string) bool {
	k := strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
	for _, part := range []string{"TOKEN", "SECRET", "PASSWORD", "PASSWD", "APIKEY", "API_KEY"} {
		if strings.Contains(k, part) {
			return true
		}
	}
	return k == "KEY" || strings.HasSuffix(k, "_KEY")
}

func isReference(v string) bool {
	return strings.HasPrefix(v, "$") || strings.Contains(v, "${") || strings.Contains(v, "{env:") || strings.Contains(v, "{file:")
}

// literalSecret reports whether v looks like a real credential rather than a
// reference, placeholder or path. The entropy floor keeps out values such as
// "your-token-here" or "changeme-changeme".
func literalSecret(v string) bool {
	if len(v) < 16 || isReference(v) || strings.ContainsAny(v, " \t<>") {
		return false
	}
	if strings.HasPrefix(v, "/") || strings.HasPrefix(v, ".") || strings.HasPrefix(v, "~") || strings.Contains(v, "://") {
		return false
	}
	return entropy(v) >= 3.5
}

func entropy(s string) float64 {
	counts := map[rune]int{}
	n := 0
	for _, r := range s {
		counts[r]++
		n++
	}
	var h float64
	for _, c := range counts {
		p := float64(c) / float64(n)
		h -= p * math.Log2(p)
	}
	return h
}

func redact(v string) string {
	if len(v) < 12 {
		return "..."
	}
	return v[:4] + "..."
}

type secretHit struct {
	kind  string
	value string
}

// findSecrets returns the secret literals in text. key is the name the text is
// stored under (an env var or header), or empty for free-form text such as a
// command line, where NAME=value assignments are inspected instead.
func findSecrets(key, text string) []secretHit {
	var hits []secretHit
	seen := map[string]bool{}
	add := func(kind, v string) {
		if !seen[v] {
			seen[v] = true
			hits = append(hits, secretHit{kind, v})
		}
	}
	for _, p := range tokenPatterns {
		for _, m := range p.re.FindAllString(text, -1) {
			add(p.kind, m)
		}
	}
	if len(hits) > 0 {
		return hits
	}
	if key != "" {
		v := text
		if scheme, rest, ok := strings.Cut(text, " "); ok && (strings.EqualFold(scheme, "Bearer") || strings.EqualFold(scheme, "token")) {
			v, key = rest, "TOKEN"
		}
		if secretKeyName(key) && literalSecret(v) {
			add("secret", v)
		}
		return hits
	}
	for _, m := range assignmentRe.FindAllStringSubmatch(text, -1) {
		v := m[2] + m[3] + m[4]
		if secretKeyName(m[1]) && literalSecret(v) {
			add("secret", v)
		}
	}
	return hits
}

func secretMessage(h secretHit, where string) string {
	return fmt.Sprintf("%s literal (%s) in %s; reference an environment variable such as ${NAME} instead of committing the value", h.kind, redact(h.value), where)
}

func checkSecrets(in inputs) []Finding {
	var out []Finding
	for _, s := range in.servers {
		for _, k := range sortedKeys(s.env) {
			for _, h := range findSecrets(k, s.env[k]) {
				out = append(out, serverFinding("SCAN-001", SeverityHigh, s, s.env[k], secretMessage(h, "env "+k)))
			}
		}
		for _, k := range sortedKeys(s.headers) {
			for _, h := range findSecrets(k, s.headers[k]) {
				out = append(out, serverFinding("SCAN-001", SeverityHigh, s, s.headers[k], secretMessage(h, "header "+k)))
			}
		}
		for _, arg := range append([]string{s.command}, s.args...) {
			for _, h := range findSecrets("", arg) {
				out = append(out, serverFinding("SCAN-001", SeverityHigh, s, arg, secretMessage(h, "command line")))
			}
		}
	}
	for _, hc := range in.hooks {
		for _, h := range findSecrets("", hc.command) {
			out = append(out, hookFinding("SCAN-001", SeverityHigh, hc, h.value, secretMessage(h, "hook command")))
		}
	}
	for _, st := range in.settings {
		for _, k := range sortedKeys(st.env) {
			for _, h := range findSecrets(k, st.env[k]) {
				out = append(out, Finding{ID: "SCAN-001", Severity: SeverityHigh, File: st.file, Subject: "settings env", Message: secretMessage(h, "env "+k), locator: st.env[k]})
			}
		}
	}
	return out
}

var remoteExecPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b(?:curl|wget)\b[^|;&]*\|\s*(?:sudo\s+)?(?:sh|bash|zsh|dash|ksh|fish|python[0-9.]*|node|perl|ruby)\b`),
	regexp.MustCompile("\\b(?:sh|bash|zsh|dash|eval|source)\\b[^;|&]*(?:\\$\\(|<\\(|`)\\s*(?:curl|wget)\\b"),
}

func remoteExec(cmd string) string {
	for _, re := range remoteExecPatterns {
		if m := re.FindString(cmd); m != "" {
			return m
		}
	}
	return ""
}

func remoteExecMessage(fragment string) string {
	return fmt.Sprintf("downloads and executes code at runtime (%q); whatever the remote serves runs with your permissions. Vendor the script or pin it by checksum", truncate(fragment, 80))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func checkRemoteExec(in inputs) []Finding {
	var out []Finding
	for _, s := range in.servers {
		if m := remoteExec(s.commandLine()); m != "" {
			locator := s.command
			for _, a := range s.args {
				if strings.Contains(a, "curl") || strings.Contains(a, "wget") {
					locator = a
				}
			}
			out = append(out, serverFinding("SCAN-002", SeverityHigh, s, locator, remoteExecMessage(m)))
		}
	}
	for _, h := range in.hooks {
		if m := remoteExec(h.command); m != "" {
			out = append(out, hookFinding("SCAN-002", SeverityHigh, h, m, remoteExecMessage(m)))
		}
	}
	return out
}

var commandSeparators = map[string]bool{"|": true, "||": true, "&&": true, ";": true, "&": true}

var uvxValueFlags = map[string]bool{"--with": true, "--python": true, "-p": true, "--index": true, "--index-url": true, "--extra-index-url": true, "--with-requirements": true}

// unpinnedRunners returns "<runner> <spec>" for every npx, bunx or uvx call in
// cmd whose package is not pinned to an exact version. npx counts only with
// -y/--yes: without it npx prefers a locally installed package.
func unpinnedRunners(cmd string) []string {
	tokens := strings.Fields(cmd)
	var out []string
	for i, t := range tokens {
		runner := path.Base(strings.Trim(t, `"'`))
		if runner != "npx" && runner != "bunx" && runner != "uvx" {
			continue
		}
		var spec string
		yes := false
		for j := i + 1; j < len(tokens) && !commandSeparators[tokens[j]]; j++ {
			a := strings.Trim(tokens[j], `"'`)
			switch {
			case a == "-y" || a == "--yes":
				yes = true
			case (a == "-p" || a == "--package") && runner != "uvx", a == "--from" && runner == "uvx":
				if j+1 < len(tokens) {
					spec = strings.Trim(tokens[j+1], `"'`)
					j++
				}
			case strings.HasPrefix(a, "--package=") && runner != "uvx":
				spec = strings.TrimPrefix(a, "--package=")
			case strings.HasPrefix(a, "--from=") && runner == "uvx":
				spec = strings.TrimPrefix(a, "--from=")
			case runner == "uvx" && uvxValueFlags[a]:
				j++
			case strings.HasPrefix(a, "-"):
			default:
				if spec == "" {
					spec = a
				}
				j = len(tokens)
			}
		}
		if spec == "" || (runner == "npx" && !yes) {
			continue
		}
		if !versionPinned(spec, runner == "uvx") {
			out = append(out, runner+" "+spec)
		}
	}
	return out
}

func versionPinned(spec string, python bool) bool {
	if python && strings.Contains(spec, "==") {
		return true
	}
	at := strings.LastIndex(spec, "@")
	if at <= 0 || at == len(spec)-1 {
		return false
	}
	v := strings.TrimPrefix(spec[at+1:], "v")
	return v != "" && v[0] >= '0' && v[0] <= '9'
}

func unpinnedMessage(call string) string {
	runner, spec, _ := strings.Cut(call, " ")
	return fmt.Sprintf("%s installs %q at launch without an exact version, so a new or compromised release runs without review; pin it (for example pkg@1.2.3)", runner, spec)
}

func checkUnpinnedRunner(in inputs) []Finding {
	var out []Finding
	for _, s := range in.servers {
		for _, call := range unpinnedRunners(s.commandLine()) {
			_, spec, _ := strings.Cut(call, " ")
			out = append(out, serverFinding("SCAN-003", SeverityMedium, s, spec, unpinnedMessage(call)))
		}
	}
	for _, h := range in.hooks {
		for _, call := range unpinnedRunners(h.command) {
			_, spec, _ := strings.Cut(call, " ")
			out = append(out, hookFinding("SCAN-003", SeverityMedium, h, spec, unpinnedMessage(call)))
		}
	}
	return out
}

// Claude Code passes the tool call to hooks as JSON on stdin, so "tool_input"
// in a command is the marker that model-controlled data is being expanded.
var toolInputExecPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\beval\b`),
	regexp.MustCompile(`tool_input[^|;&]*\|\s*(?:sudo\s+)?(?:sh|bash|zsh|dash)\b`),
	regexp.MustCompile("\\b(?:sh|bash|zsh|dash)\\s+-c\\s+[\"']?(?:\\$\\(|`)"),
}

func checkHookInjection(in inputs) []Finding {
	var out []Finding
	for _, h := range in.hooks {
		if !strings.Contains(h.command, "tool_input") {
			continue
		}
		executes := false
		for _, re := range toolInputExecPatterns {
			if re.MatchString(h.command) {
				executes = true
				break
			}
		}
		switch {
		case executes:
			out = append(out, hookFinding("SCAN-004", SeverityHigh, h, "tool_input",
				"hook passes tool input to a shell for execution; a crafted tool call runs arbitrary commands"))
		case unquotedToolInput(h.command):
			out = append(out, hookFinding("SCAN-004", SeverityMedium, h, "tool_input",
				"hook expands tool input without double quotes; word splitting and globbing let a crafted path inject extra arguments. Quote the substitution"))
		}
	}
	return out
}

// unquotedToolInput reports a $(...) or `...` substitution that mentions
// tool_input, sits outside any quotes, and is not the right-hand side of a
// plain assignment (where the shell does not split words).
func unquotedToolInput(cmd string) bool {
	var quote byte
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case quote == '\'':
			if c == '\'' {
				quote = 0
			}
		case c == '\\':
			i++
		case quote == '"':
			if c == '"' {
				quote = 0
			} else if c == '$' && i+1 < len(cmd) && cmd[i+1] == '(' {
				i = closingParen(cmd, i+1)
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '$' && i+1 < len(cmd) && cmd[i+1] == '(':
			end := closingParen(cmd, i+1)
			if strings.Contains(cmd[i:end+1], "tool_input") && !afterAssignment(cmd, i) {
				return true
			}
			i = end
		case c == '`':
			end := strings.IndexByte(cmd[i+1:], '`')
			if end < 0 {
				return false
			}
			end += i + 1
			if strings.Contains(cmd[i:end], "tool_input") && !afterAssignment(cmd, i) {
				return true
			}
			i = end
		}
	}
	return false
}

// closingParen returns the index of the parenthesis closing the one at open,
// skipping quoted text, or len(s)-1 when it is unbalanced.
func closingParen(s string, open int) int {
	depth := 0
	var quote byte
	for i := open; i < len(s); i++ {
		c := s[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '\'' || c == '"':
			quote = c
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return len(s) - 1
}

var assignmentPrefixRe = regexp.MustCompile(`(?:^|[\s;&|])[A-Za-z_][A-Za-z0-9_]*=$`)

func afterAssignment(cmd string, i int) bool {
	return assignmentPrefixRe.MatchString(cmd[:i])
}

func checkIgnoredProjectSettings(in inputs) []Finding {
	var out []Finding
	for _, s := range in.settings {
		if s.defaultMode == "bypassPermissions" {
			out = append(out, Finding{
				ID: "SCAN-005", Severity: SeverityLow, File: s.file, Subject: "permissions.defaultMode",
				Message: `"bypassPermissions" in project settings has been ignored by Claude Code since 2026-09-02; sessions start in the default mode, so do not rely on it`,
				locator: "bypassPermissions",
			})
		}
	}
	return out
}

func localHost(host string) bool {
	h := strings.ToLower(host)
	if h == "localhost" || strings.HasSuffix(h, ".localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && (ip.IsLoopback() || ip.IsUnspecified())
}

func checkPlainHTTP(in inputs) []Finding {
	var out []Finding
	for _, s := range in.servers {
		if s.url == "" {
			continue
		}
		u, err := url.Parse(s.url)
		if err != nil || !strings.EqualFold(u.Scheme, "http") || localHost(u.Hostname()) {
			continue
		}
		out = append(out, serverFinding("SCAN-006", SeverityMedium, s, s.url,
			fmt.Sprintf("connects to %s over plain http; requests and any auth headers are readable on the network. Use https", u.Host)))
	}
	return out
}
