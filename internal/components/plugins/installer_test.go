package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/cursor"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// ─── Interface compliance ───────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	inst := New(nil, nil)
	if inst.ID() != domain.ComponentPlugins {
		t.Errorf("ID() = %q, want %q", inst.ID(), domain.ComponentPlugins)
	}
}

// ─── Plan ───────────────────────────────────────────────────────────────────

func TestPlan_ClaudePlugin_ForClaude_PlansUpdate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestPluginInstaller("claude_plugin")

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionCreate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionCreate)
	}
	if actions[0].Description != "plugin:claude:test-plugin" {
		t.Errorf("Description = %q, want plugin:claude:test-plugin", actions[0].Description)
	}
	// Target should be the settings path.
	expected := adapter.SettingsPath(home)
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestPlan_ClaudePlugin_ForOpenCode_Skipped(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	// Plugin only supports "claude-code" agent.
	plugins := map[string]domain.PluginDef{
		"test-plugin": {
			Enabled:         true,
			SupportedAgents: []string{"claude-code"},
			InstallMethod:   "claude_plugin",
			PluginID:        "test@official",
		},
	}
	inst := New(plugins, &domain.MergedConfig{})

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions (not a supported agent), got %d", len(actions))
	}
}

func TestPlan_SkillFiles_ForOpenCode_Planned(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	plugins := map[string]domain.PluginDef{
		"superpowers": {
			Enabled:         true,
			SupportedAgents: []string{"opencode"},
			InstallMethod:   "skill_files",
		},
	}
	inst := New(plugins, &domain.MergedConfig{})

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionCreate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionCreate)
	}
	expectedPath := filepath.Join(project, ".opencode", "skills", "superpowers", "SKILL.md")
	if actions[0].TargetPath != expectedPath {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expectedPath)
	}
}

func TestPlan_DisabledPlugin_Skipped(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	plugins := map[string]domain.PluginDef{
		"test-plugin": {
			Enabled:         false,
			SupportedAgents: []string{"claude-code"},
			InstallMethod:   "claude_plugin",
			PluginID:        "test@official",
		},
	}
	inst := New(plugins, &domain.MergedConfig{})

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for disabled plugin, got %d", len(actions))
	}
}

func TestPlan_EmptyPlugins_ReturnsNil(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := New(nil, nil)

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for empty plugins, got %d", len(actions))
	}
}

func TestPlan_UnsupportedAdapter_ReturnsNil(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := vscode.New() // VS Code doesn't support plugins
	inst := newTestPluginInstaller("claude_plugin")

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for unsupported adapter, got %d", len(actions))
	}
}

func TestPlan_ExcludedMethodology_Skipped(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	plugins := map[string]domain.PluginDef{
		"superpowers": {
			Enabled:             true,
			SupportedAgents:     []string{"claude-code"},
			InstallMethod:       "claude_plugin",
			PluginID:            "superpowers@official",
			ExcludesMethodology: "tdd",
		},
	}
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyTDD}
	inst := New(plugins, cfg)

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions when methodology is excluded, got %d", len(actions))
	}
}

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_ClaudePlugin_CreatesEnabledPlugins(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestPluginInstaller("claude_plugin")

	actions, _ := inst.Plan(adapter, home, project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, err := os.ReadFile(actions[0].TargetPath)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	plugins, ok := doc["enabledPlugins"].(map[string]interface{})
	if !ok {
		t.Fatal("enabledPlugins should be a map")
	}
	val, ok := plugins["test@official"]
	if !ok {
		t.Error("test@official should be present in enabledPlugins")
	}
	if val != true {
		t.Errorf("test@official = %v, want true", val)
	}
}

func TestApply_ClaudePlugin_PreservesExistingSettings(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestPluginInstaller("claude_plugin")

	// Write existing settings.
	settingsPath := adapter.SettingsPath(home)
	writeTestJSON(t, settingsPath, map[string]interface{}{
		"existingSetting": "value",
	})

	actions, _ := inst.Plan(adapter, home, project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSONHelper(t, settingsPath)
	if doc["existingSetting"] != "value" {
		t.Error("existing setting should be preserved")
	}
	if _, ok := doc["enabledPlugins"].(map[string]interface{}); !ok {
		t.Error("enabledPlugins should be written")
	}
}

func TestApply_SkillFiles_WritesFile(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	plugins := map[string]domain.PluginDef{
		"superpowers": {
			Enabled:         true,
			SupportedAgents: []string{"opencode"},
			InstallMethod:   "skill_files",
		},
	}
	inst := New(plugins, &domain.MergedConfig{})

	actions, _ := inst.Plan(adapter, home, project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, err := os.ReadFile(actions[0].TargetPath)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}

	expected := pluginSkillContent["superpowers"]
	if string(data) != expected {
		t.Errorf("content = %q, want %q", string(data), expected)
	}
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestVerify_ClaudePlugin_PassesAfterApply(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestPluginInstaller("claude_plugin")

	actions, _ := inst.Plan(adapter, home, project)
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatal(err)
		}
	}

	results, err := inst.Verify(adapter, home, project)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
	if len(results) == 0 {
		t.Error("expected at least 1 verify result")
	}
}

func TestVerify_ClaudePlugin_FailsWhenMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestPluginInstaller("claude_plugin")

	results, _ := inst.Verify(adapter, home, project)
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("should fail when plugin settings file missing")
	}
}

func TestVerify_SkillFiles_PassesAfterApply(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := cursor.New()
	plugins := map[string]domain.PluginDef{
		"superpowers": {
			Enabled:         true,
			SupportedAgents: []string{"cursor"},
			InstallMethod:   "skill_files",
		},
	}
	inst := New(plugins, &domain.MergedConfig{})

	actions, _ := inst.Plan(adapter, home, project)
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatal(err)
		}
	}

	results, err := inst.Verify(adapter, home, project)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
}

func TestVerify_SkillFiles_FailsWhenMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	plugins := map[string]domain.PluginDef{
		"superpowers": {
			Enabled:         true,
			SupportedAgents: []string{"opencode"},
			InstallMethod:   "skill_files",
		},
	}
	inst := New(plugins, &domain.MergedConfig{})

	results, _ := inst.Verify(adapter, home, project)
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("should fail when skill file missing")
	}
}

// ─── effectiveInstallMethod ─────────────────────────────────────────────────

func TestEffectiveInstallMethod(t *testing.T) {
	claudeAdapter := claude.New()
	openCodeAdapter := opencode.New()

	tests := []struct {
		name    string
		plugin  domain.PluginDef
		adapter domain.Adapter
		want    string
	}{
		{
			name:    "claude_plugin on Claude Code stays claude_plugin",
			plugin:  domain.PluginDef{InstallMethod: "claude_plugin"},
			adapter: claudeAdapter,
			want:    "claude_plugin",
		},
		{
			name:    "claude_plugin on OpenCode falls back to skill_files",
			plugin:  domain.PluginDef{InstallMethod: "claude_plugin"},
			adapter: openCodeAdapter,
			want:    "skill_files",
		},
		{
			name:    "skill_files on Claude Code stays skill_files",
			plugin:  domain.PluginDef{InstallMethod: "skill_files"},
			adapter: claudeAdapter,
			want:    "skill_files",
		},
		{
			name:    "skill_files on OpenCode stays skill_files",
			plugin:  domain.PluginDef{InstallMethod: "skill_files"},
			adapter: openCodeAdapter,
			want:    "skill_files",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := effectiveInstallMethod(tc.plugin, tc.adapter)
			if got != tc.want {
				t.Errorf("effectiveInstallMethod() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ─── OpenCode + claude_plugin fallback ──────────────────────────────────────

// TestPluginInstaller_OpenCode_ClaudePlugin_FallsToSkillFiles verifies that a
// plugin with InstallMethod "claude_plugin" is treated as "skill_files" when
// the adapter is OpenCode. This prevents the installer from writing an
// "enabledPlugins" key that OpenCode does not recognise.
func TestPluginInstaller_OpenCode_ClaudePlugin_FallsToSkillFiles(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	plugins := map[string]domain.PluginDef{
		"superpowers": {
			Enabled:         true,
			SupportedAgents: []string{"opencode", "claude-code"},
			InstallMethod:   "claude_plugin",
			PluginID:        "superpowers@claude-plugins-official",
		},
	}
	inst := New(plugins, &domain.MergedConfig{})

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	action := actions[0]

	// Must use the skill_files description prefix, not claude_plugin.
	if !strings.HasPrefix(action.Description, "plugin:skill:") {
		t.Errorf("Description = %q, want prefix plugin:skill:", action.Description)
	}

	// Target path must be inside the project skills dir, not a settings.json.
	expectedPath := filepath.Join(project, ".opencode", "skills", "superpowers", "SKILL.md")
	if action.TargetPath != expectedPath {
		t.Errorf("TargetPath = %q, want %q", action.TargetPath, expectedPath)
	}

	// Applying the action must NOT write "enabledPlugins" anywhere.
	if action.Action != domain.ActionSkip {
		if err := inst.Apply(action); err != nil {
			t.Fatalf("Apply failed: %v", err)
		}
		// Confirm the settings file was NOT created.
		settingsPath := adapter.SettingsPath(home)
		if _, err := os.Stat(settingsPath); err == nil {
			t.Errorf("settings file %q should not have been created for OpenCode", settingsPath)
		}
		// Confirm the skill file was created instead.
		if _, err := os.Stat(action.TargetPath); err != nil {
			t.Errorf("skill file %q should have been created: %v", action.TargetPath, err)
		}
	}
}

// TestPluginInstaller_ClaudeCode_ClaudePlugin_Unchanged verifies that a plugin
// with InstallMethod "claude_plugin" is still handled as claude_plugin when the
// adapter is Claude Code.
func TestPluginInstaller_ClaudeCode_ClaudePlugin_Unchanged(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()

	plugins := map[string]domain.PluginDef{
		"superpowers": {
			Enabled:         true,
			SupportedAgents: []string{"claude-code"},
			InstallMethod:   "claude_plugin",
			PluginID:        "superpowers@claude-plugins-official",
		},
	}
	inst := New(plugins, &domain.MergedConfig{})

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}

	action := actions[0]

	// Must retain the claude_plugin description prefix.
	if !strings.HasPrefix(action.Description, "plugin:claude:") {
		t.Errorf("Description = %q, want prefix plugin:claude:", action.Description)
	}

	// Target path must point to Claude Code's settings.json.
	expectedPath := adapter.SettingsPath(home)
	if action.TargetPath != expectedPath {
		t.Errorf("TargetPath = %q, want %q", action.TargetPath, expectedPath)
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func newTestPluginInstaller(method string) *Installer {
	plugins := map[string]domain.PluginDef{
		"test-plugin": {
			Enabled:         true,
			SupportedAgents: []string{"claude-code"},
			InstallMethod:   method,
			PluginID:        "test@official",
		},
	}
	return New(plugins, &domain.MergedConfig{})
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

func readTestJSONHelper(t *testing.T, path string) map[string]interface{} {
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
