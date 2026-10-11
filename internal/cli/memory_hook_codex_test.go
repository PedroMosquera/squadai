package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/exitcode"
)

// Codex Stop contract, codex-rs/hooks/src/events/stop.rs and schema.rs in
// github.com/openai/codex: exit 0 with {"decision":"block","reason":...}
// records reason as a new prompt and continues the turn, and the next Stop of
// that turn carries stop_hook_active true. The output struct denies unknown
// fields, so Claude's hookSpecificOutput would fail the hook.

func codexStopPayload(t *testing.T, sessionID string, lastMessage any, stopHookActive bool) string {
	t.Helper()
	cwd, _ := os.Getwd()
	data, err := json.Marshal(map[string]any{
		"session_id": sessionID, "turn_id": "turn-1", "transcript_path": nil, "cwd": cwd,
		"hook_event_name": "Stop", "model": "gpt-5", "permission_mode": "default",
		"stop_hook_active": stopHookActive, "last_assistant_message": lastMessage,
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runCodexMemoryHook(t *testing.T, payload string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := RunMemoryHook([]string{"--harness", "codex", "--event", "Stop"}, strings.NewReader(payload), &out)
	return out.String(), err
}

// codexContinuation returns the reason of a Codex Stop block, failing unless
// the response is exactly {"decision":"block","reason":"..."}.
func codexContinuation(t *testing.T, stdout string) string {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%q", err, stdout)
	}
	reason, _ := resp["reason"].(string)
	if len(resp) != 2 || resp["decision"] != "block" || strings.TrimSpace(reason) == "" {
		t.Fatalf("want exactly decision block and a non-empty reason, got %s", stdout)
	}
	return reason
}

func TestMemoryHookCodex_SessionIsAskedOnceWithABlockDecision(t *testing.T) {
	seedMemoryHookProject(t, memoryHookSeed{captureOnStop: true, memoryComponent: true})

	out, err := runCodexMemoryHook(t, codexStopPayload(t, "019a-codex-a", "Done.", false))
	if err != nil {
		t.Fatalf("err = %v, want exit 0", err)
	}
	reason := codexContinuation(t, out)
	for _, want := range []string{"memory_add", "squadai memory add"} {
		if !strings.Contains(reason, want) {
			t.Errorf("reason lacks %q:\n%s", want, reason)
		}
	}

	if out, err := runCodexMemoryHook(t, codexStopPayload(t, "019a-codex-a", "Done.", false)); err != nil || out != "" {
		t.Errorf("second Stop in the same session: err %v, stdout %q; want exit 0 and no output", err, out)
	}
	if out, _ := runCodexMemoryHook(t, codexStopPayload(t, "019a-codex-b", "Done.", false)); out == "" {
		t.Errorf("a different session was not asked")
	}
}

func TestMemoryHookCodex_StopHookActiveStaysSilent(t *testing.T) {
	seedMemoryHookProject(t, memoryHookSeed{captureOnStop: true, memoryComponent: true})

	if out, err := runCodexMemoryHook(t, codexStopPayload(t, "019a-codex-a", "Saved two notes.", true)); err != nil || out != "" {
		t.Errorf("stop_hook_active: err %v, stdout %q; want exit 0 and no output", err, out)
	}
}

func TestMemoryHookCodex_NoAssistantMessageStaysSilentWithoutUsingTheAsk(t *testing.T) {
	seedMemoryHookProject(t, memoryHookSeed{captureOnStop: true, memoryComponent: true})

	for _, msg := range []any{nil, "   "} {
		if out, err := runCodexMemoryHook(t, codexStopPayload(t, "019a-codex-a", msg, false)); err != nil || out != "" {
			t.Fatalf("last_assistant_message %v: err %v, stdout %q; want exit 0 and no output", msg, err, out)
		}
	}
	if out, _ := runCodexMemoryHook(t, codexStopPayload(t, "019a-codex-a", "Done.", false)); out == "" {
		t.Errorf("session was not asked once it produced a reply; the silent Stops must not use up the ask")
	}
}

func TestMemoryHookCodex_InertWhenCaptureOffOrMemoryDisabled(t *testing.T) {
	for name, seed := range map[string]memoryHookSeed{
		"capture_on_stop false": {captureOnStop: false, memoryComponent: true},
		"memory disabled":       {captureOnStop: true, memoryComponent: false},
	} {
		t.Run(name, func(t *testing.T) {
			seedMemoryHookProject(t, seed)
			if out, err := runCodexMemoryHook(t, codexStopPayload(t, "019a-codex-a", "Done.", false)); err != nil || out != "" {
				t.Errorf("err %v, stdout %q; want exit 0 and no output", err, out)
			}
		})
	}
}

func TestMemoryHook_UnknownHarnessFailsWithoutBlocking(t *testing.T) {
	seedMemoryHookProject(t, memoryHookSeed{captureOnStop: true, memoryComponent: true})
	var out bytes.Buffer
	err := RunMemoryHook([]string{"--harness", "pi", "--event", "Stop"}, strings.NewReader("{}"), &out)
	var ae *exitcode.AppError
	if err == nil || errors.As(err, &ae) || out.Len() != 0 {
		t.Errorf("err = %v, stdout %q; want a plain error (exit 1, non-blocking) and no output", err, out.String())
	}
}
