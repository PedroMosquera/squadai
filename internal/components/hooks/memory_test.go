package hooks

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
)

func stopCommands(cfg map[string][]domain.HookMatcher) []string {
	var out []string
	for _, m := range cfg["Stop"] {
		for _, h := range m.Hooks {
			out = append(out, h.Command)
		}
	}
	return out
}

func hasCommand(cmds []string, want string) bool {
	for _, c := range cmds {
		if c == want {
			return true
		}
	}
	return false
}

func TestMemoryHooks_StopEntryOnlyWhenCaptureOnStop(t *testing.T) {
	if got := MemoryHooks(false); len(got) != 0 {
		t.Errorf("MemoryHooks(false) = %+v, want nothing", got)
	}
	got := MemoryHooks(true)
	if len(got) != 1 || !hasCommand(stopCommands(got), "squadai memory-hook --event Stop") {
		t.Fatalf("MemoryHooks(true) = %+v, want exactly one Stop entry running squadai memory-hook --event Stop", got)
	}
	if e := got["Stop"][0].Hooks[0]; e.Timeout <= 0 || e.OnFailure != "" {
		t.Errorf("Stop entry = %+v, want a timeout and no onFailure (a broken capture hook must not keep Claude working)", e)
	}
}

func TestApply_MemoryStopHookSitsBesideBudgetStopAndUserStopHook(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"hooks": map[string]any{
			"Stop": []any{map[string]any{
				"hooks": []any{map[string]any{"type": "command", "command": "./notify.sh"}},
			}},
		},
	})

	applyDesired(t, dir, WithMemoryHooks(WithBudgetHooks(nil, budgetUsage("warn")), true))

	_, got := readHooks(t, dir)
	cmds := stopCommands(got)
	for _, want := range []string{"./notify.sh", "squadai budget-hook --event Stop", "squadai memory-hook --event Stop"} {
		if !hasCommand(cmds, want) {
			t.Errorf("Stop hooks %q, missing %q", cmds, want)
		}
	}
}

func TestApply_CaptureOffRemovesAnEarlierMemoryHookOnly(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"hooks": map[string]any{
			"Stop": []any{map[string]any{
				"hooks": []any{map[string]any{"type": "command", "command": "./notify.sh"}},
			}},
		},
	})
	applyDesired(t, dir, WithMemoryHooks(nil, true))
	if _, before := readHooks(t, dir); !hasCommand(stopCommands(before), "squadai memory-hook --event Stop") {
		t.Fatalf("precondition: memory hook not installed, Stop hooks %q", stopCommands(before))
	}

	applyDesired(t, dir, WithMemoryHooks(nil, false))

	doc, got := readHooks(t, dir)
	raw, _ := json.Marshal(doc)
	if strings.Contains(string(raw), "memory-hook") {
		t.Errorf("memory hook left behind after capture was turned off:\n%s", raw)
	}
	if !hasCommand(stopCommands(got), "./notify.sh") {
		t.Errorf("user Stop hook dropped:\n%s", raw)
	}
}
