package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/exitcode"
)

// Stop hook contract, https://code.claude.com/docs/en/hooks: the payload
// carries stop_hook_active, true when Claude is already continuing because of
// a Stop hook; hookSpecificOutput.additionalContext keeps the conversation
// going as hook feedback; exit 0 with no output lets Claude stop.

type memoryHookSeed struct {
	captureOnStop   bool
	memoryComponent bool
	toolCalls       int
}

// seedMemoryHookProject writes project.json and a transcript with the given
// number of tool calls under a temp HOME and project, and returns the
// transcript path.
func seedMemoryHookProject(t *testing.T, s memoryHookSeed) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAUDE_PROJECT_DIR", "")
	project := t.TempDir()
	t.Chdir(project)
	cfg := map[string]any{
		"components": map[string]any{"memory": map[string]any{"enabled": s.memoryComponent}},
		"memory":     map[string]any{"backend": "native", "capture_on_stop": s.captureOnStop},
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(project, ".squadai"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".squadai", "project.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	transcript := filepath.Join(t.TempDir(), "session.jsonl")
	writeToolCalls(t, transcript, s.toolCalls)
	return transcript
}

func writeToolCalls(t *testing.T, transcript string, n int) {
	t.Helper()
	var b strings.Builder
	b.WriteString(`{"type":"user","message":{"role":"user","content":"fix the flaky test"}}` + "\n")
	for i := 0; i < n; i++ {
		b.WriteString(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"t","name":"Read","input":{"file_path":"a.go"}}]}}` + "\n")
		b.WriteString(`{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t","content":"ok"}]}}` + "\n")
	}
	b.WriteString(`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Done."}]}}` + "\n")
	if err := os.WriteFile(transcript, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

func stopPayload(t *testing.T, sessionID, transcript string, stopHookActive bool) string {
	t.Helper()
	cwd, _ := os.Getwd()
	data, err := json.Marshal(map[string]any{
		"session_id": sessionID, "transcript_path": transcript, "cwd": cwd,
		"permission_mode": "default", "hook_event_name": "Stop",
		"stop_hook_active": stopHookActive, "last_assistant_message": "Done.",
	})
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func runMemoryHook(t *testing.T, payload string) (string, int) {
	t.Helper()
	var out bytes.Buffer
	err := RunMemoryHook([]string{"--event", "Stop"}, strings.NewReader(payload), &out)
	var ae *exitcode.AppError
	switch {
	case err == nil:
		return out.String(), exitcode.OK
	case errors.As(err, &ae):
		return out.String(), ae.Code
	default:
		return out.String(), exitcode.Unexpected
	}
}

// captureContext returns the additionalContext of a Stop response, failing
// if the response is not exactly that shape.
func captureContext(t *testing.T, stdout string) string {
	t.Helper()
	var resp struct {
		Decision           string `json:"decision"`
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(stdout), &resp); err != nil {
		t.Fatalf("stdout is not one JSON object: %v\n%q", err, stdout)
	}
	if resp.Decision != "" {
		t.Errorf("response carries decision %q; capture is guidance, not an error", resp.Decision)
	}
	if resp.HookSpecificOutput.HookEventName != "Stop" {
		t.Errorf("hookEventName = %q, want Stop", resp.HookSpecificOutput.HookEventName)
	}
	return resp.HookSpecificOutput.AdditionalContext
}

func TestMemoryHook_NonTrivialSessionIsAskedOnceThenAllowed(t *testing.T) {
	transcript := seedMemoryHookProject(t, memoryHookSeed{captureOnStop: true, memoryComponent: true, toolCalls: 12})

	out, code := runMemoryHook(t, stopPayload(t, "sess-a", transcript, false))
	if code != exitcode.OK {
		t.Fatalf("exit %d, want 0", code)
	}
	ctx := captureContext(t, out)
	for _, want := range []string{"memory_add", "squadai memory add"} {
		if !strings.Contains(ctx, want) {
			t.Errorf("additionalContext lacks %q:\n%s", want, ctx)
		}
	}

	out, code = runMemoryHook(t, stopPayload(t, "sess-a", transcript, false))
	if code != exitcode.OK || out != "" {
		t.Errorf("second Stop in the same session: exit %d, stdout %q; want exit 0 and no output", code, out)
	}

	out, _ = runMemoryHook(t, stopPayload(t, "sess-b", transcript, false))
	if out == "" {
		t.Errorf("a different session was not asked")
	}
}

func TestMemoryHook_StopHookActiveAllows(t *testing.T) {
	transcript := seedMemoryHookProject(t, memoryHookSeed{captureOnStop: true, memoryComponent: true, toolCalls: 12})

	out, code := runMemoryHook(t, stopPayload(t, "sess-a", transcript, true))
	if code != exitcode.OK || out != "" {
		t.Errorf("stop_hook_active: exit %d, stdout %q; want exit 0 and no output", code, out)
	}
}

func TestMemoryHook_TrivialSessionAllowsUntilItGrows(t *testing.T) {
	transcript := seedMemoryHookProject(t, memoryHookSeed{captureOnStop: true, memoryComponent: true, toolCalls: 2})

	out, code := runMemoryHook(t, stopPayload(t, "sess-a", transcript, false))
	if code != exitcode.OK || out != "" {
		t.Fatalf("trivial session: exit %d, stdout %q; want exit 0 and no output", code, out)
	}

	writeToolCalls(t, transcript, 12)
	out, _ = runMemoryHook(t, stopPayload(t, "sess-a", transcript, false))
	if out == "" {
		t.Errorf("session that grew past the threshold was not asked; the trivial Stop must not use up the once-per-session prompt")
	}
}

func TestMemoryHook_InertWhenCaptureOffOrMemoryDisabled(t *testing.T) {
	for name, seed := range map[string]memoryHookSeed{
		"capture_on_stop false": {captureOnStop: false, memoryComponent: true, toolCalls: 12},
		"memory disabled":       {captureOnStop: true, memoryComponent: false, toolCalls: 12},
	} {
		t.Run(name, func(t *testing.T) {
			transcript := seedMemoryHookProject(t, seed)
			out, code := runMemoryHook(t, stopPayload(t, "sess-a", transcript, false))
			if code != exitcode.OK || out != "" {
				t.Errorf("exit %d, stdout %q; want exit 0 and no output", code, out)
			}
		})
	}
}

func TestMemoryHook_UnknownEventFailsWithoutBlocking(t *testing.T) {
	seedMemoryHookProject(t, memoryHookSeed{captureOnStop: true, memoryComponent: true, toolCalls: 12})
	var out bytes.Buffer
	err := RunMemoryHook([]string{"--event", "PreToolUse"}, strings.NewReader("{}"), &out)
	var ae *exitcode.AppError
	if err == nil || errors.As(err, &ae) {
		t.Errorf("err = %v, want a plain error (exit 1, non-blocking)", err)
	}
}
