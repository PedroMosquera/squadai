package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setProjectKey replaces one top-level key of the project's project.json.
func setProjectKey(t *testing.T, project, key string, value any) {
	t.Helper()
	path := filepath.Join(project, ".squadai", "project.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	doc[key] = value
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func setCaptureOnStop(t *testing.T, project string, on bool) {
	t.Helper()
	setProjectKey(t, project, "memory", map[string]any{"backend": "native", "export_path": "docs/memory", "capture_on_stop": on})
}

func stopHookCommands(t *testing.T, project string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(project, ".claude", "settings.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("settings.json: %v\n%s", err, data)
	}
	var cmds []string
	for _, m := range doc.Hooks["Stop"] {
		for _, h := range m.Hooks {
			cmds = append(cmds, h.Command)
		}
	}
	return cmds
}

func runApplyOK(t *testing.T) {
	t.Helper()
	var out bytes.Buffer
	if err := RunApply([]string{"--no-review", "--json"}, &out); err != nil {
		t.Fatalf("RunApply: %v\n%s", err, out.String())
	}
}

func TestRunApply_CaptureOnStopInstallsBesideBudgetAndUserStopHooksAndScansClean(t *testing.T) {
	project := setupClaudeCommandsProject(t)
	setBudgetUsage(t, project, map[string]any{"daily_token_budget": 1000, "enforcement": "warn"})
	setCaptureOnStop(t, project, true)
	userHook := `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"./notify.sh"}]}]}}`
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".claude", "settings.json"), []byte(userHook), 0o644); err != nil {
		t.Fatal(err)
	}

	runApplyOK(t)

	cmds := stopHookCommands(t, project)
	for _, want := range []string{"./notify.sh", "squadai budget-hook --event Stop", "squadai memory-hook --event Stop"} {
		found := false
		for _, c := range cmds {
			found = found || c == want
		}
		if !found {
			t.Errorf("Stop hooks %q, missing %q", cmds, want)
		}
	}

	var scanOut bytes.Buffer
	_ = RunScan([]string{"--json"}, &scanOut)
	var report struct {
		Findings []map[string]any `json:"findings"`
	}
	if err := json.Unmarshal(scanOut.Bytes(), &report); err != nil {
		t.Fatalf("scan output is not JSON: %v\n%s", err, scanOut.String())
	}
	for _, f := range report.Findings {
		raw, _ := json.Marshal(f)
		if strings.Contains(string(raw), "settings.json") || strings.Contains(string(raw), "memory-hook") {
			t.Errorf("scan flags the generated settings: %s", raw)
		}
	}
}

func TestRunApply_CaptureOnStopOffInstallsNothingAndRemovesEarlierInstall(t *testing.T) {
	project := setupClaudeCommandsProject(t)
	setCaptureOnStop(t, project, false)
	runApplyOK(t)
	if cmds := stopHookCommands(t, project); hasMemoryHook(cmds) {
		t.Fatalf("capture_on_stop false installed the memory hook: %q", cmds)
	}

	setCaptureOnStop(t, project, true)
	runApplyOK(t)
	if cmds := stopHookCommands(t, project); !hasMemoryHook(cmds) {
		t.Fatalf("capture_on_stop true: Stop hooks %q, want the memory hook", cmds)
	}

	setCaptureOnStop(t, project, false)
	runApplyOK(t)
	cmds := stopHookCommands(t, project)
	if hasMemoryHook(cmds) {
		t.Errorf("turning capture_on_stop off left the memory hook: %q", cmds)
	}
	if len(cmds) != 1 || cmds[0] != "squadai budget-hook --event Stop" {
		t.Errorf("Stop hooks %q, want the default budget hook kept", cmds)
	}
}

func hasMemoryHook(cmds []string) bool {
	for _, c := range cmds {
		if strings.HasPrefix(c, "squadai memory-hook ") {
			return true
		}
	}
	return false
}

func TestRunApply_CaptureOnStopNeedsTheMemoryComponent(t *testing.T) {
	project := setupClaudeCommandsProject(t)
	setCaptureOnStop(t, project, true)
	data, err := os.ReadFile(filepath.Join(project, ".squadai", "project.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	components, _ := doc["components"].(map[string]any)
	if components == nil {
		components = map[string]any{}
	}
	components["memory"] = map[string]any{"enabled": false}
	setProjectKey(t, project, "components", components)

	runApplyOK(t)
	if cmds := stopHookCommands(t, project); hasMemoryHook(cmds) {
		t.Errorf("memory component disabled but the memory hook is installed: %q", cmds)
	}
}

func TestRunApply_CaptureOnStopInstallsCodexHookBesideBudgetAndUserHooksAndRemovesIt(t *testing.T) {
	project := setupCodexPiProject(t)
	setBudgetUsage(t, project, map[string]any{"daily_token_budget": 1000, "enforcement": "block"})
	setCaptureOnStop(t, project, true)
	if err := os.MkdirAll(filepath.Join(project, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".codex", "hooks.json"), []byte(userCodexHooks), 0o644); err != nil {
		t.Fatal(err)
	}

	applyNoReview(t)

	stop := strings.Join(codexHookCommands(t, project)["Stop"], "\n")
	for _, want := range []string{"./notify.sh", "squadai budget-hook --harness codex --event Stop", "squadai memory-hook --harness codex --event Stop"} {
		if !strings.Contains(stop, want) {
			t.Errorf(".codex/hooks.json Stop hooks lack %q:\n%s", want, stop)
		}
	}
	var scanOut bytes.Buffer
	_ = RunScan([]string{"--json"}, &scanOut)
	var report struct {
		Findings []map[string]any `json:"findings"`
	}
	if err := json.Unmarshal(scanOut.Bytes(), &report); err != nil {
		t.Fatalf("scan output is not JSON: %v\n%s", err, scanOut.String())
	}
	if len(report.Findings) != 0 {
		t.Errorf("scan flags the generated config: %v", report.Findings)
	}

	setCaptureOnStop(t, project, false)
	applyNoReview(t)
	stop = strings.Join(codexHookCommands(t, project)["Stop"], "\n")
	if strings.Contains(stop, "memory-hook") {
		t.Errorf("turning capture_on_stop off left the Codex memory hook:\n%s", stop)
	}
	for _, want := range []string{"./notify.sh", "squadai budget-hook --harness codex --event Stop"} {
		if !strings.Contains(stop, want) {
			t.Errorf("removing the memory hook dropped %q:\n%s", want, stop)
		}
	}
}
