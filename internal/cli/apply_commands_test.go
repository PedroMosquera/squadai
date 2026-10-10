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

// squadaiCommandNames lists the slash commands that the removed
// install-commands command used to write into .claude/commands/.
var squadaiCommandNames = []string{
	"squadai-plan", "squadai-apply", "squadai-verify", "squadai-status",
	"squadai-doctor", "squadai-context", "squadai-init",
	"memory-add", "memory-search", "memory-promote", "memory-reindex",
}

// installCommandsBytes returns exactly what install-commands wrote for name,
// so adoption tests reproduce real existing installs byte for byte.
func installCommandsBytes(t *testing.T, name string) string {
	t.Helper()
	content, err := assets.Read("commands/" + name + ".md")
	if err != nil {
		t.Fatalf("read asset %s: %v", name, err)
	}
	return content + "\n"
}

// setupClaudeCommandsProject initialises a Claude-only project with the
// commands component enabled (via a methodology) and returns its directory.
func setupClaudeCommandsProject(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)
	t.Chdir(project)
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatalf("create claude config dir: %v", err)
	}
	var initOut bytes.Buffer
	if err := RunInit([]string{"--agents=claude-code", "--methodology=tdd", "--json"}, &initOut); err != nil {
		t.Fatalf("RunInit: %v\n%s", err, initOut.String())
	}
	return project
}

func planActions(t *testing.T) []domain.PlannedAction {
	t.Helper()
	var out bytes.Buffer
	if err := RunPlan([]string{"--json"}, &out); err != nil {
		t.Fatalf("RunPlan: %v\n%s", err, out.String())
	}
	var actions []domain.PlannedAction
	if err := json.Unmarshal(out.Bytes(), &actions); err != nil {
		t.Fatalf("parse plan json: %v\n%s", err, out.String())
	}
	return actions
}

func TestRunApply_Claude_WritesSquadaiCommands(t *testing.T) {
	project := setupClaudeCommandsProject(t)

	var applyOut bytes.Buffer
	if err := RunApply([]string{"--no-review", "--json"}, &applyOut); err != nil {
		t.Fatalf("RunApply: %v\n%s", err, applyOut.String())
	}

	commandsDir := filepath.Join(project, ".claude", "commands")
	for _, name := range squadaiCommandNames {
		got, err := os.ReadFile(filepath.Join(commandsDir, name+".md"))
		if err != nil {
			t.Errorf("apply should write %s.md: %v", name, err)
			continue
		}
		if string(got) != installCommandsBytes(t, name) {
			t.Errorf("%s.md content does not match the shipped asset:\n%s", name, got)
		}
	}

	var verifyOut bytes.Buffer
	if err := RunVerify(nil, &verifyOut); err != nil {
		t.Fatalf("RunVerify: %v\n%s", err, verifyOut.String())
	}
	if strings.Contains(verifyOut.String(), "[FAIL]") {
		t.Fatalf("verify should pass after apply:\n%s", verifyOut.String())
	}
}

func TestRunApply_Claude_AdoptsInstallCommandsFiles(t *testing.T) {
	project := setupClaudeCommandsProject(t)

	commandsDir := filepath.Join(project, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range squadaiCommandNames {
		if err := os.WriteFile(filepath.Join(commandsDir, name+".md"), []byte(installCommandsBytes(t, name)), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	byPath := map[string]domain.PlannedAction{}
	for _, a := range planActions(t) {
		byPath[a.TargetPath] = a
	}
	for _, name := range squadaiCommandNames {
		path := filepath.Join(commandsDir, name+".md")
		a, ok := byPath[path]
		if !ok {
			t.Errorf("plan should manage %s.md, found no action for it", name)
			continue
		}
		if a.Action != domain.ActionSkip || a.Component != domain.ComponentCommands {
			t.Errorf("%s.md: want commands skip (adopted as-is), got %s %s", name, a.Component, a.Action)
		}
	}

	var applyOut bytes.Buffer
	if err := RunApply([]string{"--no-review", "--json"}, &applyOut); err != nil {
		t.Fatalf("RunApply: %v\n%s", err, applyOut.String())
	}
	for _, name := range squadaiCommandNames {
		got, err := os.ReadFile(filepath.Join(commandsDir, name+".md"))
		if err != nil || string(got) != installCommandsBytes(t, name) {
			t.Errorf("%s.md should be left byte-identical by apply (err=%v)", name, err)
		}
	}
}

func TestRunApply_CommandsDisabled_RemovesOnlySquadaiCommands(t *testing.T) {
	project := setupClaudeCommandsProject(t)

	var applyOut bytes.Buffer
	if err := RunApply([]string{"--no-review", "--json"}, &applyOut); err != nil {
		t.Fatalf("first RunApply: %v\n%s", err, applyOut.String())
	}

	commandsDir := filepath.Join(project, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	userCmd := filepath.Join(commandsDir, "my-own.md")
	if err := os.WriteFile(userCmd, []byte("---\ndescription: mine\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Same name as a squadai command but edited by the user: not provably
	// ours any more, so disabling must not delete it.
	editedCmd := filepath.Join(commandsDir, "squadai-plan.md")
	if err := os.WriteFile(editedCmd, []byte("my local tweak\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	projPath := filepath.Join(project, ".squadai", "project.json")
	raw, err := os.ReadFile(projPath)
	if err != nil {
		t.Fatal(err)
	}
	var proj map[string]any
	if err := json.Unmarshal(raw, &proj); err != nil {
		t.Fatal(err)
	}
	proj["components"].(map[string]any)["commands"] = map[string]any{"enabled": false}
	raw, err = json.MarshalIndent(proj, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(projPath, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	applyOut.Reset()
	if err := RunApply([]string{"--no-review", "--json"}, &applyOut); err != nil {
		t.Fatalf("second RunApply: %v\n%s", err, applyOut.String())
	}

	for _, name := range squadaiCommandNames {
		if name == "squadai-plan" {
			continue
		}
		if _, err := os.Stat(filepath.Join(commandsDir, name+".md")); !os.IsNotExist(err) {
			t.Errorf("%s.md should be removed when the commands component is disabled (stat err=%v)", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(commandsDir, "review.md")); !os.IsNotExist(err) {
		t.Errorf("config-defined review.md should be removed when the commands component is disabled (stat err=%v)", err)
	}
	if _, err := os.Stat(userCmd); err != nil {
		t.Errorf("user-authored command must survive: %v", err)
	}
	if got, err := os.ReadFile(editedCmd); err != nil || string(got) != "my local tweak\n" {
		t.Errorf("user-edited squadai-plan.md must survive unchanged (err=%v, got %q)", err, got)
	}
}
