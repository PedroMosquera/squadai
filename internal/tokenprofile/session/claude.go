package session

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// claudeProjectsDir is the Claude Code transcript root, relative to the
// user's home directory. Each subdirectory is a project slug containing
// one .jsonl transcript per session.
const claudeProjectsDir = ".claude/projects"

// claudeScanBufferSize is the maximum accepted transcript line length.
// Claude Code writes one JSON event per line and single events (large tool
// results, pasted files) can run to several megabytes.
const claudeScanBufferSize = 10 * 1024 * 1024

// claudeEvent is the subset of a Claude Code transcript event needed for
// usage aggregation. Assistant events carry the API usage block.
type claudeEvent struct {
	Type    string `json:"type"`
	Message struct {
		Model string `json:"model"`
		Usage struct {
			InputTokens              int `json:"input_tokens"`
			OutputTokens             int `json:"output_tokens"`
			CacheReadInputTokens     int `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// ClaudeProjectSlug returns Claude Code's directory name for a project
// path: every character outside [A-Za-z0-9-] (path separators, dots,
// underscores, spaces) is replaced with a dash, so
// /Users/x/web.app becomes -Users-x-web-app. Verified against real
// ~/.claude/projects directories.
func ClaudeProjectSlug(projectDir string) string {
	var b strings.Builder
	b.Grow(len(projectDir))
	for _, r := range projectDir {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	return b.String()
}

// scanClaudeSessions walks ~/.claude/projects/<slug>/*.jsonl transcripts and
// accumulates per-model usage into agg. One .jsonl file counts as one
// session. Files older than cutoff (by ModTime) are skipped, matching
// walkSessions. When projectDir is non-empty, only the slug directory for
// that project is scanned. All errors are swallowed — a missing or
// unreadable directory never aborts aggregation.
func scanClaudeSessions(homeDir string, cutoff time.Time, projectDir string, agg *Aggregation) {
	root := filepath.Join(homeDir, claudeProjectsDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	slugFilter := ""
	if projectDir != "" {
		slugFilter = ClaudeProjectSlug(projectDir)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if slugFilter != "" && e.Name() != slugFilter {
			continue
		}
		dir := filepath.Join(root, e.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			if !cutoff.IsZero() {
				if info, err := f.Info(); err == nil && info.ModTime().Before(cutoff) {
					continue
				}
			}
			scanClaudeTranscript(filepath.Join(dir, f.Name()), agg)
		}
	}
}

// scanClaudeTranscript streams one .jsonl transcript, accumulating usage
// from assistant events. Malformed lines and non-assistant events are
// skipped silently. Each model that appears in the file counts one session.
func scanClaudeTranscript(path string, agg *Aggregation) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), claudeScanBufferSize)

	modelsSeen := map[string]bool{}
	sessionTokens := 0
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var ev claudeEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}
		if ev.Type != "assistant" {
			continue
		}
		u := ev.Message.Usage
		if u.InputTokens == 0 && u.OutputTokens == 0 &&
			u.CacheReadInputTokens == 0 && u.CacheCreationInputTokens == 0 {
			continue
		}
		model := ev.Message.Model
		if model == "" {
			model = "unknown"
		}
		acc := agg.ByModel[model]
		acc.Model = model
		acc.InputTokens += u.InputTokens
		acc.OutputTokens += u.OutputTokens
		acc.CacheReadTokens += u.CacheReadInputTokens
		acc.CacheCreationTokens += u.CacheCreationInputTokens
		if !modelsSeen[model] {
			modelsSeen[model] = true
			acc.SessionCount++
		}
		agg.ByModel[model] = acc
		sessionTokens += u.InputTokens + u.OutputTokens
	}
	if sessionTokens > agg.MaxSessionTokens {
		agg.MaxSessionTokens = sessionTokens
	}
}
