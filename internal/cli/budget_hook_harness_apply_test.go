package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupCodexPiProject(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", "")
	t.Setenv("PI_CODING_AGENT_DIR", "")
	t.Chdir(project)
	for _, d := range []string{".codex", filepath.Join(".pi", "agent")} {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var initOut bytes.Buffer
	if err := RunInit([]string{"--agents=codex,pi", "--methodology=tdd", "--json"}, &initOut); err != nil {
		t.Fatalf("RunInit: %v\n%s", err, initOut.String())
	}
	return project
}

func applyNoReview(t *testing.T) {
	t.Helper()
	var out bytes.Buffer
	if err := RunApply([]string{"--no-review", "--json"}, &out); err != nil {
		t.Fatalf("RunApply: %v\n%s", err, out.String())
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

// codexHookCommands maps each hooks.json event to the commands under it.
func codexHookCommands(t *testing.T, project string) map[string][]string {
	t.Helper()
	var doc struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command   string `json:"command"`
				OnFailure string `json:"onFailure"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	data := readFile(t, filepath.Join(project, ".codex", "hooks.json"))
	if data == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(data), &doc); err != nil {
		t.Fatalf(".codex/hooks.json: %v\n%s", err, data)
	}
	out := map[string][]string{}
	for ev, groups := range doc.Hooks {
		for _, g := range groups {
			for _, h := range g.Hooks {
				if h.OnFailure != "" {
					t.Errorf("%s %q carries onFailure, which Codex does not define", ev, h.Command)
				}
				out[ev] = append(out[ev], h.Command)
			}
		}
	}
	return out
}

const userCodexHooks = `{"hooks":{"Stop":[{"hooks":[{"type":"command","command":"./notify.sh","timeout":5}]}]}}`

func TestRunApply_BlockBudgetWritesCodexAndPiEnforcementKeepsUserHooks(t *testing.T) {
	project := setupCodexPiProject(t)
	setBudgetUsage(t, project, map[string]any{"daily_token_budget": 1000, "enforcement": "block"})
	if err := os.MkdirAll(filepath.Join(project, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".codex", "hooks.json"), []byte(userCodexHooks), 0o644); err != nil {
		t.Fatal(err)
	}

	applyNoReview(t)

	cmds := codexHookCommands(t, project)
	for _, ev := range []string{"PreToolUse", "UserPromptSubmit", "Stop"} {
		want := "squadai budget-hook --harness codex --event " + ev
		if !strings.Contains(strings.Join(cmds[ev], "\n"), want) {
			t.Errorf("%s: no %q in .codex/hooks.json: %v", ev, want, cmds)
		}
	}
	if !strings.Contains(strings.Join(cmds["Stop"], "\n"), "./notify.sh") {
		t.Errorf("user Stop hook dropped: %v", cmds)
	}

	ext := readFile(t, filepath.Join(project, ".pi", "extensions", "squadai-budget.ts"))
	for _, want := range []string{`const ENFORCEMENT = "block";`, `"--harness", "pi"`, `pi.on("tool_call"`, `pi.on("input"`, `pi.on("agent_before_settle"`} {
		if !strings.Contains(ext, want) {
			t.Errorf("pi extension lacks %q:\n%s", want, ext)
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

	// Unsetting enforcement removes what apply wrote and nothing else.
	setBudgetUsage(t, project, map[string]any{"daily_token_budget": 1000})
	applyNoReview(t)
	cmds = codexHookCommands(t, project)
	if strings.Contains(strings.Join(append(cmds["PreToolUse"], cmds["Stop"]...), "\n"), "budget-hook") {
		t.Errorf("budget hooks left in .codex/hooks.json after unsetting enforcement: %v", cmds)
	}
	if !strings.Contains(strings.Join(cmds["Stop"], "\n"), "./notify.sh") {
		t.Errorf("user Stop hook dropped on removal: %v", cmds)
	}
	if _, err := os.Stat(filepath.Join(project, ".pi", "extensions", "squadai-budget.ts")); !os.IsNotExist(err) {
		t.Errorf("pi extension still present after unsetting enforcement (stat err %v)", err)
	}
}

func TestRunApply_WarnBudgetSkipsCodexPreToolUse(t *testing.T) {
	project := setupCodexPiProject(t)
	setBudgetUsage(t, project, map[string]any{"daily_token_budget": 1000, "enforcement": "ask"})
	applyNoReview(t)
	cmds := codexHookCommands(t, project)
	if len(cmds["PreToolUse"]) != 0 || len(cmds["UserPromptSubmit"]) != 1 || len(cmds["Stop"]) != 1 {
		t.Errorf("ask on Codex should install the warn set (UserPromptSubmit, Stop): %v", cmds)
	}
}

func TestRunApply_EnforcementUnsetInstallsNoCodexOrPiEnforcement(t *testing.T) {
	project := setupCodexPiProject(t)
	setBudgetUsage(t, project, map[string]any{"daily_token_budget": 1000})
	applyNoReview(t)
	if data := readFile(t, filepath.Join(project, ".codex", "hooks.json")); data != "" {
		t.Errorf(".codex/hooks.json written with enforcement unset:\n%s", data)
	}
	if data := readFile(t, filepath.Join(project, ".pi", "extensions", "squadai-budget.ts")); data != "" {
		t.Errorf("pi extension written with enforcement unset")
	}
}
