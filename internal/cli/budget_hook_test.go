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

	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/tokenprofile/session"
)

// The response shapes asserted here follow the Claude Code hooks reference,
// https://code.claude.com/docs/en/hooks: exit 2 blocks PreToolUse and
// UserPromptSubmit with stderr as the reason, exit 0 with a JSON object on
// stdout is parsed for systemMessage and hookSpecificOutput, and a Stop
// "block" means "keep working", so a budget hook must never return it there.

// seedHookProject writes project.json with usage and one Claude transcript of
// sessionTokens (input plus output) for the project, under a temp HOME. It
// returns the hook payload Claude Code would send for that session.
func seedHookProject(t *testing.T, usage map[string]any, sessionTokens int) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	project := t.TempDir()
	t.Chdir(project)
	if usage != nil {
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
	}
	slugDir := filepath.Join(home, ".claude", "projects", session.ClaudeProjectSlug(project))
	if err := os.MkdirAll(slugDir, 0o755); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(slugDir, "current.jsonl")
	line := fmt.Sprintf(`{"type":"assistant","message":{"model":"claude-sonnet-4-5","usage":{"input_tokens":%d,"output_tokens":%d}}}`,
		sessionTokens-sessionTokens/2, sessionTokens/2)
	if err := os.WriteFile(transcript, []byte(line+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(map[string]any{
		"session_id": "test-session", "transcript_path": transcript, "cwd": project,
		"permission_mode": "default", "hook_event_name": "PreToolUse",
		"tool_name": "Bash", "tool_input": map[string]any{"command": "go test ./..."},
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func runHook(t *testing.T, event, payload string) (stdout string, code int, err error) {
	t.Helper()
	var out bytes.Buffer
	err = RunBudgetHook([]string{"--event", event}, strings.NewReader(payload), &out)
	// Mirrors cmd/squadai/main.go: a plain error exits Unexpected (1).
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

func systemMessage(t *testing.T, stdout string) string {
	t.Helper()
	var resp struct {
		SystemMessage string `json:"systemMessage"`
		Decision      string `json:"decision"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%q", err, stdout)
	}
	if resp.Decision != "" {
		t.Errorf("response carries decision %q, want none", resp.Decision)
	}
	return resp.SystemMessage
}

func TestBudgetHook_BlockOverBudgetBlocksToolsAndPrompts(t *testing.T) {
	payload := seedHookProject(t, map[string]any{"daily_token_budget": 1000, "enforcement": "block"}, 1500)
	for _, ev := range []string{"PreToolUse", "UserPromptSubmit"} {
		stdout, code, err := runHook(t, ev, payload)
		if code != 2 {
			t.Fatalf("%s: exit code %d (%v), want 2", ev, code, err)
		}
		if !strings.Contains(err.Error(), "daily token budget exceeded") || !strings.Contains(err.Error(), "1500 / 1000") {
			t.Errorf("%s: block reason %q does not name the exceeded budget", ev, err)
		}
		if stdout != "" {
			t.Errorf("%s: stdout %q, want empty (stderr carries the exit 2 reason)", ev, stdout)
		}
	}
}

func TestBudgetHook_UnderBudgetAllowsSilently(t *testing.T) {
	payload := seedHookProject(t, map[string]any{"daily_token_budget": 1000, "session_token_budget": 800, "enforcement": "block"}, 600)
	for _, ev := range []string{"PreToolUse", "UserPromptSubmit", "Stop"} {
		stdout, code, err := runHook(t, ev, payload)
		if code != exitcode.OK || stdout != "" {
			t.Errorf("%s: code %d (%v) stdout %q, want exit 0 and no output", ev, code, err, stdout)
		}
	}
}

func TestBudgetHook_SessionBudgetCountsTheCurrentTranscript(t *testing.T) {
	payload := seedHookProject(t, map[string]any{"session_token_budget": 1000, "enforcement": "block"}, 1200)
	if _, code, err := runHook(t, "PreToolUse", payload); code != 2 || !strings.Contains(err.Error(), "session token budget exceeded") {
		t.Fatalf("code %d (%v), want 2 naming the session budget", code, err)
	}
}

func TestBudgetHook_WarnNeverBlocks(t *testing.T) {
	payload := seedHookProject(t, map[string]any{"daily_token_budget": 1000, "enforcement": "warn"}, 1500)
	for _, ev := range []string{"PreToolUse", "UserPromptSubmit", "Stop"} {
		stdout, code, err := runHook(t, ev, payload)
		if code != exitcode.OK {
			t.Fatalf("%s: exit code %d (%v) under warn, want 0", ev, code, err)
		}
		if ev == "PreToolUse" {
			if stdout != "" {
				t.Errorf("PreToolUse under warn printed %q, want silence", stdout)
			}
			continue
		}
		if msg := systemMessage(t, stdout); !strings.Contains(msg, "daily token budget exceeded") {
			t.Errorf("%s: systemMessage %q, want the exceeded budget", ev, msg)
		}
	}
}

func TestBudgetHook_StopNeverBlocksEvenUnderBlock(t *testing.T) {
	payload := seedHookProject(t, map[string]any{"daily_token_budget": 1000, "enforcement": "block"}, 1500)
	stdout, code, err := runHook(t, "Stop", payload)
	if code != exitcode.OK {
		t.Fatalf("Stop exit code %d (%v), want 0", code, err)
	}
	if msg := systemMessage(t, stdout); !strings.Contains(msg, "daily token budget exceeded") {
		t.Errorf("Stop systemMessage %q, want the exceeded budget", msg)
	}
}

func TestBudgetHook_AskAsksBeforeEachToolCall(t *testing.T) {
	payload := seedHookProject(t, map[string]any{"daily_token_budget": 1000, "enforcement": "ask"}, 1500)
	stdout, code, err := runHook(t, "PreToolUse", payload)
	if code != exitcode.OK {
		t.Fatalf("exit code %d (%v), want 0", code, err)
	}
	var resp struct {
		HookSpecificOutput struct {
			HookEventName            string `json:"hookEventName"`
			PermissionDecision       string `json:"permissionDecision"`
			PermissionDecisionReason string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%q", err, stdout)
	}
	h := resp.HookSpecificOutput
	if h.HookEventName != "PreToolUse" || h.PermissionDecision != "ask" || !strings.Contains(h.PermissionDecisionReason, "daily") {
		t.Errorf("hookSpecificOutput = %+v, want PreToolUse ask naming the daily budget", h)
	}
}

func TestBudgetHook_EnforcementUnsetOrOffIsANoop(t *testing.T) {
	for _, usage := range []map[string]any{
		{"daily_token_budget": 1000},
		{"daily_token_budget": 1000, "enforcement": "off"},
	} {
		payload := seedHookProject(t, usage, 1500)
		for _, ev := range []string{"PreToolUse", "UserPromptSubmit", "Stop"} {
			stdout, code, err := runHook(t, ev, payload)
			if code != exitcode.OK || stdout != "" {
				t.Errorf("usage %v %s: code %d (%v) stdout %q, want exit 0 and no output", usage, ev, code, err, stdout)
			}
		}
	}
}

func TestBudgetHook_UnknownEventFailsWithoutBlocking(t *testing.T) {
	seedHookProject(t, nil, 0)
	_, code, err := runHook(t, "PreModelSwitch", "{}")
	if err == nil || code == exitcode.HookBlock {
		t.Fatalf("code %d (%v), want an error that is not exit 2", code, err)
	}
}
