package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// TestRunStatus_RefinementSection verifies that RunStatus includes a
// "Refinement" section in its human-readable output, and that it shows
// "never run" when .squad-refined does not exist but project.json does.
func TestRunStatus_RefinementSection(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Write a minimal project.json so the project is initialized.
	proj := domain.DefaultProjectConfig()
	projectPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.WriteJSON(projectPath, proj); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	// No .squad-refined file exists — refinement has never been run.
	var buf bytes.Buffer
	if err := RunStatus([]string{}, &buf); err != nil {
		t.Fatalf("RunStatus should not error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Refinement") {
		t.Errorf("status output should include a 'Refinement' section, got:\n%s", out)
	}
	if !strings.Contains(out, "never-refined") {
		t.Errorf("status output should say 'never-refined' when .squad-refined is absent, got:\n%s", out)
	}
}

// TestRunStatus_RefinementSection_JSON verifies that the --json output
// includes a "refinement" key with a "status" field.
func TestRunStatus_RefinementSection_JSON(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Write a minimal project.json.
	proj := domain.DefaultProjectConfig()
	projectPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.WriteJSON(projectPath, proj); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	var buf bytes.Buffer
	if err := RunStatus([]string{"--json"}, &buf); err != nil {
		t.Fatalf("RunStatus --json should not error: %v", err)
	}

	out := buf.String()
	// The JSON output should contain a "refinement" key.
	if !strings.Contains(out, `"refinement"`) {
		t.Errorf("JSON status output should include 'refinement' key, got:\n%s", out)
	}
	if !strings.Contains(out, `"status"`) {
		t.Errorf("JSON status refinement should include 'status' field, got:\n%s", out)
	}
}

func TestRunStatus_DailyIncludesControlPlaneFields(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	proj := domain.DefaultProjectConfig()
	proj.Preset = domain.PresetSoloPower
	proj.Meta = domain.ProjectMeta{Language: "Go"}
	projectPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.WriteJSON(projectPath, proj); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	var buf bytes.Buffer
	if err := RunStatus([]string{"--daily"}, &buf); err != nil {
		t.Fatalf("RunStatus --daily should not error: %v", err)
	}

	out := buf.String()
	for _, want := range []string{
		"Daily Status:",
		"preset=solo-power",
		"profile=default",
		"Memory: backend=native",
		"Usage: enforcement=warn",
		"Health: setup pending",
		"Next: run squadai apply --no-review",
		"Refinement: never-refined",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("status --daily output should contain %q, got:\n%s", want, out)
		}
	}
}

// TestRunSquadInitStatus_NeverRefined verifies that RunSquadInitStatus
// returns a JSON object with status=never-refined when .squad-refined is absent.
func TestRunSquadInitStatus_NeverRefined(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Write a minimal project.json.
	proj := domain.DefaultProjectConfig()
	projectPath := filepath.Join(dir, config.ProjectConfigDir, "project.json")
	if err := os.MkdirAll(filepath.Dir(projectPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := config.WriteJSON(projectPath, proj); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	var buf bytes.Buffer
	if err := RunSquadInitStatus([]string{}, &buf); err != nil {
		t.Fatalf("RunSquadInitStatus should not error: %v", err)
	}

	out := strings.TrimSpace(buf.String())
	if !strings.HasPrefix(out, "{") {
		t.Fatalf("expected JSON output, got: %s", out)
	}
	if !strings.Contains(out, `"status"`) {
		t.Errorf("expected 'status' field in JSON, got: %s", out)
	}
	if !strings.Contains(out, "never-refined") {
		t.Errorf("expected status=never-refined, got: %s", out)
	}
}
