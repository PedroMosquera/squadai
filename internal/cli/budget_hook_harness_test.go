package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

func runHarnessHook(t *testing.T, harness, event, payload string) (stdout string, code int, err error) {
	t.Helper()
	var out bytes.Buffer
	err = RunBudgetHook([]string{"--harness", harness, "--event", event}, strings.NewReader(payload), &out)
	var ae *exitcode.AppError
	switch {
	case err == nil:
		code = exitcode.OK
	case errors.As(err, &ae):
		code = ae.Code
	default:
		code = exitcode.Unexpected
	}
	return out.String(), code, err
}

// seedUsageProject writes project.json with usage under a temp HOME and
// returns the home and project directories.
func seedUsageProject(t *testing.T, usage map[string]any) (home, project string) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Setenv("CODEX_HOME", "")
	project = t.TempDir()
	t.Chdir(project)
	data, err := json.Marshal(map[string]any{"usage": usage})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, ".squadai"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".squadai", "project.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	return home, project
}

// seedPiSession writes one Pi session file in the layout of
// docs/session-format.md and returns the payload the squadai Pi extension
// sends for it.
func seedPiSession(t *testing.T, home, project string, tokens int) string {
	t.Helper()
	dir := session.PiSessionDir(home, project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "2026-10-11T10-00-00-000Z_test-session.jsonl")
	lines := fmt.Sprintf(`{"type":"session","version":3,"id":"test-session","cwd":%q}
{"type":"message","id":"a1","message":{"role":"user","content":"hi"}}
{"type":"message","id":"a2","message":{"role":"assistant","model":"claude-sonnet-4-5","usage":{"input":%d,"output":%d,"cacheRead":0,"cacheWrite":0,"totalTokens":%d}}}
`, project, tokens-tokens/2, tokens/2, tokens)
	if err := os.WriteFile(file, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{"cwd": project, "transcript_path": file})
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func TestBudgetHook_PiBlocksOverBudgetFromPiSessions(t *testing.T) {
	home, project := seedUsageProject(t, map[string]any{"daily_token_budget": 1000, "enforcement": "block"})
	payload := seedPiSession(t, home, project, 1500)
	for _, ev := range []string{"PreToolUse", "UserPromptSubmit"} {
		stdout, code, err := runHarnessHook(t, "pi", ev, payload)
		if code != exitcode.HookBlock {
			t.Fatalf("%s: exit code %d (%v), want 2", ev, code, err)
		}
		if !strings.Contains(err.Error(), "1500 / 1000") || stdout != "" {
			t.Errorf("%s: reason %q stdout %q, want the exceeded budget on stderr only", ev, err, stdout)
		}
	}
}

func TestBudgetHook_PiSessionBudgetCountsTheCurrentSessionFile(t *testing.T) {
	home, project := seedUsageProject(t, map[string]any{"session_token_budget": 1000, "enforcement": "block"})
	payload := seedPiSession(t, home, project, 1200)
	if _, code, err := runHarnessHook(t, "pi", "PreToolUse", payload); code != exitcode.HookBlock || !strings.Contains(err.Error(), "session token budget exceeded") {
		t.Fatalf("code %d (%v), want 2 naming the session budget", code, err)
	}
}

func TestBudgetHook_PiUnderBudgetAllowsSilently(t *testing.T) {
	home, project := seedUsageProject(t, map[string]any{"daily_token_budget": 1000, "enforcement": "block"})
	payload := seedPiSession(t, home, project, 600)
	for _, ev := range []string{"PreToolUse", "UserPromptSubmit", "Stop"} {
		if stdout, code, err := runHarnessHook(t, "pi", ev, payload); code != exitcode.OK || stdout != "" {
			t.Errorf("%s: code %d (%v) stdout %q, want exit 0 and no output", ev, code, err, stdout)
		}
	}
}

func TestBudgetHook_PiIgnoresClaudeUsage(t *testing.T) {
	_ = seedHookProject(t, map[string]any{"daily_token_budget": 1000, "enforcement": "block"}, 1500)
	project, _ := os.Getwd()
	payload := fmt.Sprintf(`{"cwd":%q}`, project)
	if _, code, err := runHarnessHook(t, "pi", "PreToolUse", payload); code != exitcode.OK {
		t.Fatalf("code %d (%v): Claude transcripts counted against the Pi budget", code, err)
	}
}

func TestBudgetHook_UnknownHarnessFailsWithoutBlocking(t *testing.T) {
	seedHookProject(t, nil, 0)
	_, code, err := runHarnessHook(t, "cursor", "PreToolUse", "{}")
	if err == nil || code == exitcode.HookBlock {
		t.Fatalf("code %d (%v), want an error that is not exit 2", code, err)
	}
}

// fakeCcusage puts a ccusage on PATH that prints a `ccusage codex session
// --json` report (shape from rust/adapters/codex/src/report.rs in
// github.com/ccusage/ccusage) with one current session of currentTokens and
// one session idle for three days that must not count toward the daily budget.
func fakeCcusage(t *testing.T, currentTokens int) {
	t.Helper()
	report := map[string]any{"sessions": []map[string]any{
		{
			"sessionId": "2026/10/11/rollout-current", "sessionFile": "rollout-current", "directory": "2026/10/11",
			"lastActivity": time.Now().UTC().Format(time.RFC3339),
			"inputTokens":  currentTokens - currentTokens/2, "outputTokens": currentTokens / 2, "totalTokens": currentTokens,
		},
		{
			"sessionId": "2026/10/08/rollout-old", "sessionFile": "rollout-old", "directory": "2026/10/08",
			"lastActivity": time.Now().Add(-72 * time.Hour).UTC().Format(time.RFC3339),
			"inputTokens":  50000, "outputTokens": 50000, "totalTokens": 100000,
		},
	}}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf '%s\\n' '" + string(data) + "'\n"
	if err := os.WriteFile(filepath.Join(bin, "ccusage"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
}

func codexPayload(project string) string {
	return fmt.Sprintf(`{"session_id":"codex-test","cwd":%q,"transcript_path":"/sessions/2026/10/11/rollout-current.jsonl","hook_event_name":"PreToolUse","tool_name":"Bash","tool_input":{"command":"ls"}}`, project)
}

func TestBudgetHook_CodexBlocksOverBudgetFromCcusage(t *testing.T) {
	_, project := seedUsageProject(t, map[string]any{"daily_token_budget": 1000, "enforcement": "block"})
	fakeCcusage(t, 1500)
	for _, ev := range []string{"PreToolUse", "UserPromptSubmit"} {
		stdout, code, err := runHarnessHook(t, "codex", ev, codexPayload(project))
		if code != exitcode.HookBlock {
			t.Fatalf("%s: exit code %d (%v), want 2", ev, code, err)
		}
		if !strings.Contains(err.Error(), "1500 / 1000") || stdout != "" {
			t.Errorf("%s: reason %q stdout %q, want the exceeded budget on stderr only", ev, err, stdout)
		}
	}
}

func TestBudgetHook_CodexSessionBudgetMatchesTheTranscript(t *testing.T) {
	_, project := seedUsageProject(t, map[string]any{"session_token_budget": 1000, "enforcement": "block"})
	fakeCcusage(t, 1200)
	if _, code, err := runHarnessHook(t, "codex", "PreToolUse", codexPayload(project)); code != exitcode.HookBlock || !strings.Contains(err.Error(), "session token budget exceeded: 1200 / 1000") {
		t.Fatalf("code %d (%v), want 2 naming the current session", code, err)
	}
}

func TestBudgetHook_CodexUnderBudgetAllowsSilently(t *testing.T) {
	_, project := seedUsageProject(t, map[string]any{"daily_token_budget": 1000, "enforcement": "block"})
	fakeCcusage(t, 600)
	for _, ev := range []string{"PreToolUse", "UserPromptSubmit", "Stop"} {
		if stdout, code, err := runHarnessHook(t, "codex", ev, codexPayload(project)); code != exitcode.OK || stdout != "" {
			t.Errorf("%s: code %d (%v) stdout %q, want exit 0 and no output", ev, code, err, stdout)
		}
	}
}

func TestBudgetHook_CodexAskNeverSendsAskDecision(t *testing.T) {
	_, project := seedUsageProject(t, map[string]any{"daily_token_budget": 1000, "enforcement": "ask"})
	fakeCcusage(t, 1500)
	stdout, code, err := runHarnessHook(t, "codex", "PreToolUse", codexPayload(project))
	if code != exitcode.OK || strings.Contains(stdout, "permissionDecision") {
		t.Fatalf("code %d (%v) stdout %q: Codex rejects permissionDecision ask", code, err, stdout)
	}
}

func TestBudgetHook_CodexWithoutUsageSourceAllowsAndWarnsOnce(t *testing.T) {
	_, project := seedUsageProject(t, map[string]any{"daily_token_budget": 1000, "enforcement": "block"})
	t.Setenv("PATH", t.TempDir())
	stdout, code, err := runHarnessHook(t, "codex", "PreToolUse", codexPayload(project))
	if code != exitcode.OK {
		t.Fatalf("first event: code %d (%v), want 0 (fail open)", code, err)
	}
	if msg := systemMessage(t, stdout); !strings.Contains(msg, "not enforced") {
		t.Errorf("first event systemMessage %q, want a not-enforced warning", msg)
	}
	stdout, code, err = runHarnessHook(t, "codex", "UserPromptSubmit", codexPayload(project))
	if code != exitcode.OK || stdout != "" {
		t.Errorf("second event: code %d (%v) stdout %q, want a silent allow", code, err, stdout)
	}
}
