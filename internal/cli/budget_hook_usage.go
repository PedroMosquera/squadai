package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

const budgetHookWindow = 24 * time.Hour

// ccusageTimeout stays under the 10s hook timeout so a slow ccusage ends in
// the fail-open path instead of a hook timeout.
const ccusageTimeout = 8 * time.Second

// harnessUsage returns the last 24h of usage as the harness recorded it, with
// MaxSessionTokens set to the current session: inside a session the session
// budget means this session, not the largest one of the day, which would lock
// every new session after one big one. A nil result with no error means the
// harness has no usage source on this machine.
func harnessUsage(harness, homeDir, projectDir string, in budgetHookInput) (*session.Aggregation, error) {
	switch harness {
	case "pi":
		dir := session.PiSessionDir(homeDir, projectDir)
		if in.TranscriptPath != "" {
			// The session file's own directory also covers --session-dir.
			dir = filepath.Dir(in.TranscriptPath)
		}
		agg := session.PiUsage(dir, budgetHookWindow)
		if in.TranscriptPath != "" {
			agg.MaxSessionTokens = session.PiSessionTokens(in.TranscriptPath)
		}
		return agg, nil
	case "codex":
		return codexUsageFromCcusage(in), nil
	default:
		agg, err := session.Aggregate(homeDir, session.AggregateOptions{Since: budgetHookWindow, ProjectDir: projectDir})
		if err != nil {
			return nil, err
		}
		if in.TranscriptPath != "" {
			agg.MaxSessionTokens = session.ClaudeTranscriptTokens(in.TranscriptPath)
		}
		return agg, nil
	}
}

// ccusageCodexSessions is the subset of `ccusage codex session --json` that
// budgets need (rust/adapters/codex/src/report.rs in github.com/ccusage/ccusage).
// inputTokens already excludes cached input, matching the Claude figure.
type ccusageCodexSessions struct {
	Sessions []struct {
		SessionFile  string `json:"sessionFile"`
		LastActivity string `json:"lastActivity"`
		InputTokens  int    `json:"inputTokens"`
		OutputTokens int    `json:"outputTokens"`
	} `json:"sessions"`
}

// codexUsageFromCcusage reads Codex usage through ccusage, because the Codex
// rollout format is undocumented. ccusage reports no project directory, so the
// daily figure counts every Codex session active in the last 24h, whole. nil
// when ccusage is missing or fails.
func codexUsageFromCcusage(in budgetHookInput) *session.Aggregation {
	bin, err := exec.LookPath("ccusage")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), ccusageTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, "codex", "session", "--json", "--offline").Output()
	if err != nil {
		return nil
	}
	var report ccusageCodexSessions
	if err := json.Unmarshal(out, &report); err != nil {
		return nil
	}

	current := strings.TrimSuffix(filepath.Base(in.TranscriptPath), ".jsonl")
	cutoff := time.Now().Add(-budgetHookWindow)
	agg := &session.Aggregation{ByModel: map[string]session.Usage{}, Period: budgetHookWindow.String()}
	for _, s := range report.Sessions {
		tokens := s.InputTokens + s.OutputTokens
		if in.TranscriptPath != "" && s.SessionFile == current {
			agg.MaxSessionTokens = tokens
		}
		// An unparseable timestamp counts, so a format change over-reports
		// rather than silently lifting the budget.
		if t, err := time.Parse(time.RFC3339, s.LastActivity); err == nil && t.Before(cutoff) {
			continue
		}
		agg.Total.InputTokens += s.InputTokens
		agg.Total.OutputTokens += s.OutputTokens
		agg.Total.TotalTokens += tokens
		agg.Total.SessionCount++
	}
	return agg
}

// warnNoUsageSourceOnce allows the event and, on the first event of a
// session, says the budget is not enforced. A marker file per session keeps
// the warning from repeating on every tool call.
func warnNoUsageSourceOnce(stdout io.Writer, harness, sessionID string) error {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	if sessionID == "" {
		sessionID = "unknown"
	}
	mark := filepath.Join(cacheDir, "squadai", "budget-hook", harness+"-"+filepath.Base(filepath.Clean("/"+sessionID)))
	if _, err := os.Stat(mark); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(mark), 0o755); err == nil {
		_ = os.WriteFile(mark, nil, 0o644)
	}
	return writeHookJSON(stdout, map[string]any{
		"systemMessage": "squadai: token budget not enforced in " + harness + ": no usage source found. Install ccusage (https://github.com/ccusage/ccusage) to enable it.",
	})
}
