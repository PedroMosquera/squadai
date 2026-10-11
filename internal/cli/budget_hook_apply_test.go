package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setBudgetUsage rewrites the usage block of the project's project.json.
func setBudgetUsage(t *testing.T, project string, usage map[string]any) {
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
	doc["usage"] = usage
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
}

func claudeSettings(t *testing.T, project string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(project, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRunApply_BlockBudgetInstallsHooksKeepsUserHookAndScansClean(t *testing.T) {
	project := setupClaudeCommandsProject(t)
	setBudgetUsage(t, project, map[string]any{"daily_token_budget": 1000, "enforcement": "block"})
	userHook := `{"hooks":{"PostToolUse":[{"matcher":"Write","hooks":[{"type":"command","command":"./fmt.sh"}]}]}}`
	if err := os.MkdirAll(filepath.Join(project, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".claude", "settings.json"), []byte(userHook), 0o644); err != nil {
		t.Fatal(err)
	}

	var applyOut bytes.Buffer
	if err := RunApply([]string{"--no-review", "--json"}, &applyOut); err != nil {
		t.Fatalf("RunApply: %v\n%s", err, applyOut.String())
	}

	settings := claudeSettings(t, project)
	var doc struct {
		Hooks map[string][]struct {
			Matcher string           `json:"matcher"`
			Hooks   []map[string]any `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal([]byte(settings), &doc); err != nil {
		t.Fatalf("settings.json: %v\n%s", err, settings)
	}
	for _, ev := range []string{"PreToolUse", "UserPromptSubmit"} {
		found := false
		for _, m := range doc.Hooks[ev] {
			for _, h := range m.Hooks {
				if h["command"] == "squadai budget-hook --event "+ev && h["onFailure"] == "block" {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s: no budget-hook entry with onFailure block:\n%s", ev, settings)
		}
	}
	if !strings.Contains(settings, `"./fmt.sh"`) {
		t.Errorf("user PostToolUse hook dropped:\n%s", settings)
	}

	var scanOut bytes.Buffer
	_ = RunScan([]string{"--json"}, &scanOut)
	var report struct {
		Findings []struct {
			ID      string `json:"id"`
			File    string `json:"file"`
			Subject string `json:"subject"`
			Message string `json:"message"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(scanOut.Bytes(), &report); err != nil {
		t.Fatalf("scan output is not JSON: %v\n%s", err, scanOut.String())
	}
	for _, f := range report.Findings {
		if strings.Contains(f.File, "settings.json") || strings.Contains(f.Subject+f.Message, "budget-hook") {
			t.Errorf("scan flags the generated settings: %+v", f)
		}
	}
}

func TestRunApply_EnforcementUnsetInstallsNoBudgetHooks(t *testing.T) {
	project := setupClaudeCommandsProject(t)
	setBudgetUsage(t, project, map[string]any{"daily_token_budget": 1000})

	var applyOut bytes.Buffer
	if err := RunApply([]string{"--no-review", "--json"}, &applyOut); err != nil {
		t.Fatalf("RunApply: %v\n%s", err, applyOut.String())
	}
	data, err := os.ReadFile(filepath.Join(project, ".claude", "settings.json"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "budget-hook") {
		t.Errorf("budget hooks installed with enforcement unset:\n%s", data)
	}
}
