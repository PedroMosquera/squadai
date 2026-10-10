package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// managerAgentBytes returns exactly what the removed install-commands command
// wrote to .claude/agents/squadai-manager.md.
func managerAgentBytes(t *testing.T) string {
	t.Helper()
	content, err := assets.Read("agents/squadai-manager.md")
	if err != nil {
		t.Fatalf("read squadai-manager asset: %v", err)
	}
	return content + "\n"
}

func editProjectJSON(t *testing.T, project string, edit func(proj map[string]any)) {
	t.Helper()
	projPath := filepath.Join(project, ".squadai", "project.json")
	raw, err := os.ReadFile(projPath)
	if err != nil {
		t.Fatal(err)
	}
	var proj map[string]any
	if err := json.Unmarshal(raw, &proj); err != nil {
		t.Fatal(err)
	}
	edit(proj)
	raw, err = json.MarshalIndent(proj, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunApply_Claude_WritesManagerAgent(t *testing.T) {
	project := setupClaudeCommandsProject(t)

	var applyOut bytes.Buffer
	if err := RunApply([]string{"--no-review", "--json"}, &applyOut); err != nil {
		t.Fatalf("RunApply: %v\n%s", err, applyOut.String())
	}

	got, err := os.ReadFile(filepath.Join(project, ".claude", "agents", "squadai-manager.md"))
	if err != nil {
		t.Fatalf("apply should write squadai-manager.md: %v", err)
	}
	if string(got) != managerAgentBytes(t) {
		t.Errorf("squadai-manager.md content does not match the shipped asset:\n%s", got)
	}

	var verifyOut bytes.Buffer
	if err := RunVerify(nil, &verifyOut); err != nil {
		t.Fatalf("RunVerify: %v\n%s", err, verifyOut.String())
	}
	if strings.Contains(verifyOut.String(), "[FAIL]") {
		t.Fatalf("verify should pass after apply:\n%s", verifyOut.String())
	}
}

func TestRunApply_Claude_AdoptsInstalledManagerAgent(t *testing.T) {
	project := setupClaudeCommandsProject(t)

	agentPath := filepath.Join(project, ".claude", "agents", "squadai-manager.md")
	if err := os.MkdirAll(filepath.Dir(agentPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(agentPath, []byte(managerAgentBytes(t)), 0o644); err != nil {
		t.Fatal(err)
	}

	var found *domain.PlannedAction
	for _, a := range planActions(t) {
		if a.TargetPath == agentPath {
			found = &a
			break
		}
	}
	if found == nil {
		t.Fatal("plan should manage squadai-manager.md, found no action for it")
	}
	if found.Action != domain.ActionSkip || found.Component != domain.ComponentAgents {
		t.Errorf("squadai-manager.md: want agents skip (adopted as-is), got %s %s", found.Component, found.Action)
	}
}

func TestRunApply_Claude_UserAgentFilesSurviveDisable(t *testing.T) {
	project := setupClaudeCommandsProject(t)

	var applyOut bytes.Buffer
	if err := RunApply([]string{"--no-review", "--json"}, &applyOut); err != nil {
		t.Fatalf("first RunApply: %v\n%s", err, applyOut.String())
	}

	userAgent := filepath.Join(project, ".claude", "agents", "my-reviewer.md")
	userContent := "---\nname: my-reviewer\ndescription: mine\n---\n"

	steps := []struct {
		name string
		edit func(proj map[string]any)
	}{
		{"agents component disabled", func(proj map[string]any) {
			proj["components"].(map[string]any)["agents"] = map[string]any{"enabled": false}
		}},
		{"claude adapter disabled", func(proj map[string]any) {
			proj["adapters"].(map[string]any)["claude-code"] = map[string]any{"enabled": false}
		}},
	}
	for _, step := range steps {
		if err := os.WriteFile(userAgent, []byte(userContent), 0o644); err != nil {
			t.Fatal(err)
		}
		editProjectJSON(t, project, step.edit)
		applyOut.Reset()
		if err := RunApply([]string{"--no-review", "--json"}, &applyOut); err != nil {
			t.Fatalf("RunApply with %s: %v\n%s", step.name, err, applyOut.String())
		}
		if got, err := os.ReadFile(userAgent); err != nil || string(got) != userContent {
			t.Errorf("with %s, user-authored my-reviewer.md must survive unchanged (err=%v, got %q)", step.name, err, got)
		}
	}
}
