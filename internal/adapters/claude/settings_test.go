package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// ─── SetAgentTeamsEnv ───────────────────────────────────────────────────────

func TestSetAgentTeamsEnv_EnableFromMissingFile(t *testing.T) {
	dir := t.TempDir()

	changed, err := SetAgentTeamsEnv(dir, true)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !changed {
		t.Error("expected changed=true on first enable")
	}

	got := readSettings(t, dir)
	envMap, _ := got["env"].(map[string]any)
	if envMap[AgentTeamsEnvVar] != "1" {
		t.Errorf("env.%s = %v, want \"1\"", AgentTeamsEnvVar, envMap[AgentTeamsEnvVar])
	}
}

func TestSetAgentTeamsEnv_EnablePreservesExistingEnvVars(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"env": map[string]any{
			"USER_VAR": "keep-me",
		},
		"otherKey": "untouched",
	})

	changed, err := SetAgentTeamsEnv(dir, true)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !changed {
		t.Error("expected changed=true when env var added")
	}

	got := readSettings(t, dir)
	envMap, _ := got["env"].(map[string]any)
	if envMap["USER_VAR"] != "keep-me" {
		t.Errorf("USER_VAR lost: got %v", envMap["USER_VAR"])
	}
	if envMap[AgentTeamsEnvVar] != "1" {
		t.Errorf("agent teams var not set")
	}
	if got["otherKey"] != "untouched" {
		t.Errorf("sibling top-level key was overwritten: %v", got["otherKey"])
	}
}

func TestSetAgentTeamsEnv_EnableIsIdempotent(t *testing.T) {
	dir := t.TempDir()

	// First enable.
	if _, err := SetAgentTeamsEnv(dir, true); err != nil {
		t.Fatal(err)
	}

	// Second enable should be no-op.
	changed, err := SetAgentTeamsEnv(dir, true)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("expected changed=false on idempotent re-enable")
	}
}

func TestSetAgentTeamsEnv_DisableRemovesOnlyOurKey(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"env": map[string]any{
			"USER_VAR":       "keep-me",
			AgentTeamsEnvVar: "1",
		},
	})

	changed, err := SetAgentTeamsEnv(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected changed=true when removing env var")
	}

	got := readSettings(t, dir)
	envMap, _ := got["env"].(map[string]any)
	if envMap["USER_VAR"] != "keep-me" {
		t.Errorf("USER_VAR removed: %v", envMap)
	}
	if _, exists := envMap[AgentTeamsEnvVar]; exists {
		t.Errorf("agent teams var still present after disable")
	}
}

func TestSetAgentTeamsEnv_DisableRemovesEmptyEnvMap(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"env": map[string]any{
			AgentTeamsEnvVar: "1",
		},
	})

	if _, err := SetAgentTeamsEnv(dir, false); err != nil {
		t.Fatal(err)
	}

	got := readSettings(t, dir)
	if _, exists := got["env"]; exists {
		t.Errorf("env key should be removed when last entry is deleted, got %v", got["env"])
	}
}

func TestSetAgentTeamsEnv_DisableIdempotent_NoFile(t *testing.T) {
	dir := t.TempDir()

	changed, err := SetAgentTeamsEnv(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("disable on missing file should be no-op")
	}

	// File should not be created.
	if _, err := os.Stat(filepath.Join(dir, ".claude", "settings.json")); !os.IsNotExist(err) {
		t.Errorf("settings.json was created on no-op disable")
	}
}

func TestSetAgentTeamsEnv_RejectsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte("{not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := SetAgentTeamsEnv(dir, true); err == nil {
		t.Error("expected error on malformed JSON")
	}
}

// ─── AgentTeamsEnabled ──────────────────────────────────────────────────────

func TestAgentTeamsEnabled_NoFile(t *testing.T) {
	dir := t.TempDir()
	got, err := AgentTeamsEnabled(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("expected false when settings.json missing")
	}
}

func TestAgentTeamsEnabled_PresentTrue(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"env": map[string]any{AgentTeamsEnvVar: "1"},
	})
	got, err := AgentTeamsEnabled(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Error("expected true when env var set to \"1\"")
	}
}

func TestAgentTeamsEnabled_PresentNonStringFalse(t *testing.T) {
	dir := t.TempDir()
	// JSON unmarshals numbers to float64; if Claude Code's settings somehow
	// got a non-string value here, we should treat it as not-enabled rather
	// than panic.
	writeSettings(t, dir, map[string]any{
		"env": map[string]any{AgentTeamsEnvVar: 1},
	})
	got, err := AgentTeamsEnabled(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("non-string value should not count as enabled")
	}
}

func TestAgentTeamsEnabled_EnvKeyMissing(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"agent": "orchestrator",
	})
	got, err := AgentTeamsEnabled(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got {
		t.Error("expected false when env key missing")
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

func writeSettings(t *testing.T, projectDir string, doc map[string]any) {
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

func readSettings(t *testing.T, projectDir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectDir, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}
