package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/managed"
)

// ─── Interface compliance ───────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	inst := New(nil)
	if inst.ID() != domain.ComponentSettings {
		t.Errorf("ID() = %q, want %q", inst.ID(), domain.ComponentSettings)
	}
}

// ─── Constructor ────────────────────────────────────────────────────────────

func TestNew_ExtractsSettings(t *testing.T) {
	adapters := map[string]domain.AdapterConfig{
		"opencode": {Enabled: true, Settings: map[string]interface{}{
			"model": "anthropic/claude-sonnet-4-5",
		}},
		"claude-code": {Enabled: true}, // no settings
	}
	inst := New(adapters)

	if len(inst.SettingsForAdapter("opencode")) != 1 {
		t.Error("should have 1 setting for opencode")
	}
	if len(inst.SettingsForAdapter("claude-code")) != 0 {
		t.Error("should have 0 settings for claude-code (empty)")
	}
}

func TestNew_NilAdapters(t *testing.T) {
	inst := New(nil)
	if len(inst.SettingsForAdapter("opencode")) != 0 {
		t.Error("nil adapters should produce no settings")
	}
}

// ─── Plan (OpenCode) ────────────────────────────────────────────────────────

func TestPlan_OpenCode_NewFile_ReturnsCreate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller("opencode", map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	})

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionCreate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionCreate)
	}
	expected := filepath.Join(project, "opencode.json")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
	if actions[0].Component != domain.ComponentSettings {
		t.Errorf("Component = %q, want %q", actions[0].Component, domain.ComponentSettings)
	}
}

func TestPlan_OpenCode_UpToDate_ReturnsSkip(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	settings := map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	}
	inst := newTestInstaller("opencode", settings)

	// Write existing file with matching content (including $schema for OpenCode).
	targetPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"$schema": "https://opencode.ai/config.json",
		"model":   "anthropic/claude-sonnet-4-5",
	})

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionSkip {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionSkip)
	}
}

func TestPlan_OpenCode_OutdatedValue_ReturnsUpdate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller("opencode", map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	})

	// Write file with old model value.
	targetPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"model": "old-model",
	})

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionUpdate)
	}
}

func TestPlan_OpenCode_MissingKey_ReturnsUpdate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller("opencode", map[string]interface{}{
		"model":    "anthropic/claude-sonnet-4-5",
		"provider": "anthropic",
	})

	// Write file with only one of the two keys.
	targetPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	})

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actions[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionUpdate)
	}
}

// ─── Plan (Claude) ──────────────────────────────────────────────────────────

func TestPlan_Claude_TargetsProjectSettings(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller("claude-code", map[string]interface{}{
		"model": "claude-sonnet-4-5",
	})

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	expected := filepath.Join(project, ".claude", "settings.json")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

// ─── Plan (no settings) ─────────────────────────────────────────────────────

func TestPlan_NoSettings_ReturnsNil(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(map[string]domain.AdapterConfig{
		"opencode": {Enabled: true}, // no settings
	})

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for empty settings, got %d", len(actions))
	}
}

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_OpenCode_CreatesFile(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	settings := map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
		"permission": map[string]interface{}{
			"edit": "allow",
			"bash": "ask",
		},
	}
	inst := newTestInstaller("opencode", settings)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Read and verify.
	doc := readTestJSON(t, actions[0].TargetPath)
	if doc["model"] != "anthropic/claude-sonnet-4-5" {
		t.Errorf("model = %v, want %q", doc["model"], "anthropic/claude-sonnet-4-5")
	}

	perm, ok := doc["permission"].(map[string]interface{})
	if !ok {
		t.Fatal("permission should be a map")
	}
	if perm["edit"] != "allow" {
		t.Errorf("permission.edit = %v, want %q", perm["edit"], "allow")
	}

	// Check that _agent_manager is NOT written into the config doc.
	if _, hasKey := doc["_agent_manager"]; hasKey {
		t.Error("_agent_manager must not appear in the config file — tracking moved to sidecar")
	}

	// Check managed keys are written to the sidecar.
	sidecarKeys, err := managed.ReadManagedKeys(project, "opencode.json")
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	if len(sidecarKeys) != 3 {
		t.Errorf("expected 3 managed keys in sidecar, got %d: %v", len(sidecarKeys), sidecarKeys)
	}
}

func TestApply_Claude_CreatesNestedDir(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller("claude-code", map[string]interface{}{
		"model": "claude-sonnet-4-5",
	})

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify .claude directory was created.
	info, err := os.Stat(filepath.Join(project, ".claude"))
	if err != nil {
		t.Fatalf("expected .claude dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Error(".claude should be a directory")
	}

	doc := readTestJSON(t, actions[0].TargetPath)
	if doc["model"] != "claude-sonnet-4-5" {
		t.Errorf("model = %v, want %q", doc["model"], "claude-sonnet-4-5")
	}
}

func TestApply_PreservesUserKeys(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller("opencode", map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	})

	// Write existing file with user-added keys.
	targetPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"$schema":      "https://opencode.ai/config.json",
		"user_setting": "preserve me",
	})

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, targetPath)
	if doc["$schema"] != "https://opencode.ai/config.json" {
		t.Error("$schema should be preserved")
	}
	if doc["user_setting"] != "preserve me" {
		t.Error("user_setting should be preserved")
	}
	if doc["model"] != "anthropic/claude-sonnet-4-5" {
		t.Error("managed model key should be written")
	}
}

func TestApply_SkipDoesNothing(t *testing.T) {
	inst := New(nil)
	err := inst.Apply(domain.PlannedAction{Action: domain.ActionSkip})
	if err != nil {
		t.Fatalf("Skip should succeed, got: %v", err)
	}
}

func TestApply_Idempotent(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	settings := map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	}
	inst := newTestInstaller("opencode", settings)

	// First apply.
	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(actions[0].TargetPath)

	// Second plan — should be Skip.
	actions2, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions2[0].Action != domain.ActionSkip {
		t.Fatalf("second plan should be Skip, got %q", actions2[0].Action)
	}

	second, _ := os.ReadFile(actions[0].TargetPath)
	if string(first) != string(second) {
		t.Error("content should not change on second plan")
	}
}

func TestApply_UpdatesOutdatedValues(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()

	// Write old values (no _agent_manager — that's now in the sidecar).
	targetPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"model":    "old-model",
		"user_key": "keep",
	})

	// Install new value.
	inst := newTestInstaller("opencode", map[string]interface{}{
		"model": "new-model",
	})
	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions[0].Action != domain.ActionUpdate {
		t.Fatalf("expected Update, got %q", actions[0].Action)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, targetPath)
	if doc["model"] != "new-model" {
		t.Error("model should be updated")
	}
	if doc["user_key"] != "keep" {
		t.Error("user_key should be preserved")
	}
}

func TestApply_DeepNestedSettings(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller("opencode", map[string]interface{}{
		"permission": map[string]interface{}{
			"edit":   "allow",
			"bash":   "ask",
			"mcp":    "allow",
			"read":   "allow",
			"write":  "allow",
			"delete": "deny",
		},
	})

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, actions[0].TargetPath)
	perm, ok := doc["permission"].(map[string]interface{})
	if !ok {
		t.Fatal("permission should be a map")
	}
	if perm["edit"] != "allow" {
		t.Errorf("permission.edit = %v, want allow", perm["edit"])
	}
	if perm["delete"] != "deny" {
		t.Errorf("permission.delete = %v, want deny", perm["delete"])
	}
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestVerify_OpenCode_AllPass_AfterApply(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller("opencode", map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	})

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	results, err := inst.Verify(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
	if len(results) != 2 {
		t.Errorf("expected 2 verify checks, got %d", len(results))
	}
}

func TestVerify_FailsWhenFileIsMissing(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller("opencode", map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	})

	results, err := inst.Verify(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("expected settings-file-exists check to fail")
	}
}

func TestVerify_FailsWhenKeysOutdated(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller("opencode", map[string]interface{}{
		"model": "new-model",
	})

	// Write file with old value (no _agent_manager — tracking is now in the sidecar).
	targetPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"model": "old-model",
	})

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	foundKeysCheck := false
	for _, r := range results {
		if r.Check == "settings-keys-current" {
			foundKeysCheck = true
			if r.Passed {
				t.Error("settings-keys-current should fail when values are outdated")
			}
		}
	}
	if !foundKeysCheck {
		t.Error("expected settings-keys-current check in results")
	}
}

func TestVerify_NoSettings_ReturnsNil(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(nil)

	results, err := inst.Verify(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty settings, got %d", len(results))
	}
}

// ─── managedKeysMatch ───────────────────────────────────────────────────────

func TestManagedKeysMatch_AllMatch(t *testing.T) {
	doc := map[string]interface{}{
		"model":    "x",
		"provider": "y",
	}
	expected := map[string]interface{}{
		"model":    "x",
		"provider": "y",
	}
	if !managedKeysMatch(doc, expected) {
		t.Error("should match when all keys equal")
	}
}

func TestManagedKeysMatch_MissingKey(t *testing.T) {
	doc := map[string]interface{}{
		"model": "x",
	}
	expected := map[string]interface{}{
		"model":    "x",
		"provider": "y",
	}
	if managedKeysMatch(doc, expected) {
		t.Error("should not match when key is missing")
	}
}

func TestManagedKeysMatch_DifferentValue(t *testing.T) {
	doc := map[string]interface{}{
		"model": "old",
	}
	expected := map[string]interface{}{
		"model": "new",
	}
	if managedKeysMatch(doc, expected) {
		t.Error("should not match when value differs")
	}
}

func TestManagedKeysMatch_DeepNested(t *testing.T) {
	doc := map[string]interface{}{
		"permission": map[string]interface{}{"edit": "allow"},
	}
	expected := map[string]interface{}{
		"permission": map[string]interface{}{"edit": "allow"},
	}
	if !managedKeysMatch(doc, expected) {
		t.Error("should match nested objects")
	}
}

// ─── OpenCode $schema injection ─────────────────────────────────────────────

func TestApply_OpenCode_InjectsSchema(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller("opencode", map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	})

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, actions[0].TargetPath)
	schema, ok := doc["$schema"]
	if !ok {
		t.Fatal("$schema key should be present in OpenCode config")
	}
	if schema != "https://opencode.ai/config.json" {
		t.Errorf("$schema = %v, want %q", schema, "https://opencode.ai/config.json")
	}
}

func TestApply_Claude_NoSchemaInjected(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller("claude-code", map[string]interface{}{
		"model": "claude-sonnet-4-5",
	})

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, actions[0].TargetPath)
	if _, ok := doc["$schema"]; ok {
		t.Error("$schema should NOT be injected for Claude Code")
	}
}

func TestPlan_OpenCode_MissingSchema_ReturnsUpdate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller("opencode", map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	})

	// Write file with matching model but no $schema.
	targetPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	})

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actions[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q (missing $schema should trigger update)", actions[0].Action, domain.ActionUpdate)
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func newTestInstaller(adapterID string, settings map[string]interface{}) *Installer {
	return New(map[string]domain.AdapterConfig{
		adapterID: {Enabled: true, Settings: settings},
	})
}

func writeTestJSON(t *testing.T, path string, data map[string]interface{}) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
}

func readTestJSON(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
