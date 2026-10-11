package session

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// piEntry is the subset of a Pi session entry that carries usage, per
// https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/session-format.md:
// message entries hold it on the message (assistant turns, and tool results
// that did nested model work); usage, compaction and branch_summary entries
// hold it at the top level.
type piEntry struct {
	Type    string   `json:"type"`
	Model   string   `json:"model"`
	Usage   *piUsage `json:"usage"`
	Message struct {
		Model string   `json:"model"`
		Usage *piUsage `json:"usage"`
	} `json:"message"`
}

type piUsage struct {
	Input  int `json:"input"`
	Output int `json:"output"`
}

// PiSessionDir returns the directory Pi stores a project's sessions in by
// default: <agentDir>/sessions/--<cwd with the leading separator dropped and
// separators and colons replaced by dashes>--. agentDir is
// PI_CODING_AGENT_DIR when set, else ~/.pi/agent. A session dir chosen with
// --session-dir or the sessionDir setting is not discoverable from here.
func PiSessionDir(homeDir, projectDir string) string {
	agentDir := os.Getenv("PI_CODING_AGENT_DIR")
	if agentDir == "" {
		agentDir = filepath.Join(homeDir, ".pi", "agent")
	}
	safe := projectDir
	if strings.HasPrefix(safe, "/") || strings.HasPrefix(safe, `\`) {
		safe = safe[1:]
	}
	safe = strings.NewReplacer("/", "-", `\`, "-", ":", "-").Replace(safe)
	return filepath.Join(agentDir, "sessions", "--"+safe+"--")
}

// PiSessionTokens returns the input plus output tokens recorded in one Pi
// session file, the per-session figure MaxSessionTokens tracks. An unreadable
// file counts as zero.
func PiSessionTokens(path string) int {
	agg := &Aggregation{ByModel: map[string]Usage{}}
	scanPiSession(path, agg)
	return agg.MaxSessionTokens
}

// PiUsage aggregates every Pi session file in sessionDir modified within
// since. One .jsonl file counts as one session.
func PiUsage(sessionDir string, since time.Duration) *Aggregation {
	agg := &Aggregation{ByModel: map[string]Usage{}, Period: since.String()}
	cutoff := time.Now().Add(-since)
	files, err := os.ReadDir(sessionDir)
	if err == nil {
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			if info, err := f.Info(); err != nil || info.ModTime().Before(cutoff) {
				continue
			}
			scanPiSession(filepath.Join(sessionDir, f.Name()), agg)
		}
	}
	finishTotals(agg)
	return agg
}

func scanPiSession(path string, agg *Aggregation) {
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
		var e piEntry
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			continue
		}
		u, model := e.Usage, e.Model
		if e.Type == "message" {
			u, model = e.Message.Usage, e.Message.Model
		}
		if u == nil || (u.Input == 0 && u.Output == 0) {
			continue
		}
		if model == "" {
			model = "unknown"
		}
		acc := agg.ByModel[model]
		acc.Model = model
		acc.InputTokens += u.Input
		acc.OutputTokens += u.Output
		if !modelsSeen[model] {
			modelsSeen[model] = true
			acc.SessionCount++
		}
		agg.ByModel[model] = acc
		sessionTokens += u.Input + u.Output
	}
	if sessionTokens > agg.MaxSessionTokens {
		agg.MaxSessionTokens = sessionTokens
	}
}

// finishTotals fills each model's TotalTokens and the grand total, the fields
// budget checks read. Cost is left to Aggregate.
func finishTotals(agg *Aggregation) {
	agg.Total = Usage{Model: "total"}
	for model, u := range agg.ByModel {
		u.TotalTokens = u.InputTokens + u.OutputTokens
		agg.ByModel[model] = u
		agg.Total.InputTokens += u.InputTokens
		agg.Total.OutputTokens += u.OutputTokens
		agg.Total.TotalTokens += u.TotalTokens
		agg.Total.SessionCount += u.SessionCount
	}
}
