package doctor

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
)

// ─── checkAgentTeams ────────────────────────────────────────────────────────

func TestCheckAgentTeams_NoProjectConfig_Skip(t *testing.T) {
	dir := t.TempDir()
	d := &Doctor{projectDir: dir}

	r := d.checkAgentTeams()
	if r.Status != CheckSkip {
		t.Errorf("status = %v (msg=%q), want CheckSkip", r.Status, r.Message)
	}
}

func TestCheckAgentTeams_DesiredOnAndPresent_Pass(t *testing.T) {
	dir := t.TempDir()
	writeProjectConfigWithAgentTeams(t, dir, true)
	writeClaudeSettings(t, dir, map[string]any{
		"env": map[string]any{claude.AgentTeamsEnvVar: "1"},
	})

	d := &Doctor{projectDir: dir}
	r := d.checkAgentTeams()
	if r.Status != CheckPass {
		t.Errorf("status = %v (msg=%q), want CheckPass", r.Status, r.Message)
	}
}

func TestCheckAgentTeams_DesiredOffAndAbsent_Pass(t *testing.T) {
	dir := t.TempDir()
	writeProjectConfigWithAgentTeams(t, dir, false)

	d := &Doctor{projectDir: dir}
	r := d.checkAgentTeams()
	if r.Status != CheckPass {
		t.Errorf("status = %v (msg=%q), want CheckPass", r.Status, r.Message)
	}
}

func TestCheckAgentTeams_DesiredOnButMissing_Warn(t *testing.T) {
	dir := t.TempDir()
	writeProjectConfigWithAgentTeams(t, dir, true)
	// No claude settings written → drift.

	d := &Doctor{projectDir: dir}
	r := d.checkAgentTeams()
	if r.Status != CheckWarn {
		t.Errorf("status = %v (msg=%q), want CheckWarn", r.Status, r.Message)
	}
	if r.FixHint == "" {
		t.Error("expected a fix hint on drift")
	}
}

func TestCheckAgentTeams_DesiredOffButPresent_Warn(t *testing.T) {
	dir := t.TempDir()
	writeProjectConfigWithAgentTeams(t, dir, false)
	writeClaudeSettings(t, dir, map[string]any{
		"env": map[string]any{claude.AgentTeamsEnvVar: "1"},
	})

	d := &Doctor{projectDir: dir}
	r := d.checkAgentTeams()
	if r.Status != CheckWarn {
		t.Errorf("status = %v (msg=%q), want CheckWarn", r.Status, r.Message)
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

func writeProjectConfigWithAgentTeams(t *testing.T, projectDir string, enabled bool) {
	t.Helper()
	cfgDir := filepath.Join(projectDir, ".squadai")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{
		"version": 1,
		"components": map[string]any{
			"memory": map[string]any{"enabled": true},
		},
		"copilot": map[string]any{"instructions_template": "standard"},
		"claude": map[string]any{
			"agent_teams": map[string]any{"enabled": enabled},
		},
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "project.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}

func writeClaudeSettings(t *testing.T, projectDir string, doc map[string]any) {
	t.Helper()
	dir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}
