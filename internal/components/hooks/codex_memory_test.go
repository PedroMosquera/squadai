package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/codex"
	"github.com/PedroMosquera/squadai/internal/domain"
)

const codexMemoryCommand = "squadai memory-hook --harness codex --event Stop"

func applyCodex(t *testing.T, dir string, inst *Installer) {
	t.Helper()
	actions, err := inst.Plan(codex.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatal(err)
		}
	}
}

func readCodexHooks(t *testing.T, dir string) (string, map[string][]domain.HookMatcher) {
	t.Helper()
	data, err := os.ReadFile(CodexHooksPath(dir))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	return string(data), claude.DecodeHooksMap(doc["hooks"])
}

func TestCodexMemoryHooks_StopEntryOnlyWhenCaptureOnStop(t *testing.T) {
	if got := CodexMemoryHooks(false); len(got) != 0 {
		t.Errorf("CodexMemoryHooks(false) = %+v, want nothing", got)
	}
	got := CodexMemoryHooks(true)
	if len(got) != 1 || !hasCommand(stopCommands(got), codexMemoryCommand) {
		t.Fatalf("CodexMemoryHooks(true) = %+v, want exactly one Stop entry running %s", got, codexMemoryCommand)
	}
	if e := got["Stop"][0].Hooks[0]; e.Timeout <= 0 || e.OnFailure != "" {
		t.Errorf("Stop entry = %+v, want a timeout and no onFailure, which Codex does not define", e)
	}
}

func TestApply_CodexMemoryHookSitsBesideBudgetAndUserHookAndIsRemovedAlone(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(CodexHooksPath(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	user := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"./notify.sh","timeout":5}]}]}}`
	if err := os.WriteFile(CodexHooksPath(dir), []byte(user), 0o644); err != nil {
		t.Fatal(err)
	}

	applyCodex(t, dir, New(nil).WithUsage(budgetUsage("block")).WithMemoryCapture(true))
	_, got := readCodexHooks(t, dir)
	for _, want := range []string{"./notify.sh", "squadai budget-hook --harness codex --event Stop", codexMemoryCommand} {
		if !hasCommand(stopCommands(got), want) {
			t.Errorf("Stop hooks %q, missing %q", stopCommands(got), want)
		}
	}
	if res := New(nil).WithUsage(budgetUsage("block")).WithMemoryCapture(true).verifyCodexBudget(dir); len(res) != 1 || !res[0].Passed {
		t.Errorf("verify after install = %+v, want one passing result", res)
	}

	applyCodex(t, dir, New(nil).WithUsage(budgetUsage("block")))
	raw, got := readCodexHooks(t, dir)
	if strings.Contains(raw, "memory-hook") {
		t.Errorf("memory hook left behind after capture was turned off:\n%s", raw)
	}
	for _, want := range []string{"./notify.sh", "squadai budget-hook --harness codex --event Stop"} {
		if !hasCommand(stopCommands(got), want) {
			t.Errorf("Stop hooks %q, missing %q after removing the memory hook", stopCommands(got), want)
		}
	}
}

func TestApply_CodexMemoryHookWithoutBudgetCreatesAndDeletesTheFile(t *testing.T) {
	dir := t.TempDir()

	applyCodex(t, dir, New(nil).WithMemoryCapture(true))
	raw, got := readCodexHooks(t, dir)
	if len(got) != 1 || !hasCommand(stopCommands(got), codexMemoryCommand) {
		t.Fatalf("want only the memory Stop hook, got:\n%s", raw)
	}
	if res := New(nil).verifyCodexBudget(dir); len(res) != 0 {
		t.Errorf("verify with nothing desired = %+v, want no results", res)
	}
	if res := New(nil).WithMemoryCapture(true).verifyCodexBudget(dir); len(res) != 1 || !res[0].Passed {
		t.Errorf("verify with capture on = %+v, want one passing result", res)
	}

	applyCodex(t, dir, New(nil))
	if raw, _ := readCodexHooks(t, dir); raw != "" {
		t.Errorf(".codex/hooks.json kept after the only squadai hook was removed:\n%s", raw)
	}
}
