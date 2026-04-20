package plugins

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// Compile-time assertion that Installer implements domain.Previewer.
var _ domain.Previewer = (*Installer)(nil)

func TestPreview_ClaudePlugin_FreshInstall_CreateEntryWithDiff(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestPluginInstaller("claude_plugin")

	entries, err := inst.Preview(adapter, home, project)
	if err != nil {
		t.Fatalf("Preview() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Action != domain.ActionCreate {
		t.Errorf("Action = %q, want %q", e.Action, domain.ActionCreate)
	}
	if e.Component != domain.ComponentPlugins {
		t.Errorf("Component = %q, want %q", e.Component, domain.ComponentPlugins)
	}
	if !strings.Contains(e.Diff, "enabledPlugins") || !strings.Contains(e.Diff, "test@official") {
		t.Errorf("diff should reference enabledPlugins and the plugin id, got:\n%s", e.Diff)
	}
	if len(e.Conflicts) != 0 {
		t.Errorf("plugins previewer never reports conflicts, got %v", e.Conflicts)
	}
}

func TestPreview_ClaudePlugin_AlreadyEnabled_SkipWithEmptyDiff(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestPluginInstaller("claude_plugin")

	target := adapter.SettingsPath(home)
	writeTestJSON(t, target, map[string]interface{}{
		"enabledPlugins": map[string]interface{}{
			"test@official": true,
		},
	})

	entries, err := inst.Preview(adapter, home, project)
	if err != nil {
		t.Fatalf("Preview() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Action != domain.ActionSkip {
		t.Errorf("Action = %q, want %q", entries[0].Action, domain.ActionSkip)
	}
	if entries[0].Diff != "" {
		t.Errorf("skip entry should have empty diff, got:\n%s", entries[0].Diff)
	}
}

func TestPreview_ClaudePlugin_PreservesUserEnabledPlugins_NoConflict(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestPluginInstaller("claude_plugin")

	target := adapter.SettingsPath(home)
	// User has other plugins enabled — the diff should ADD our plugin without
	// removing theirs; no conflict is surfaced because Apply only mutates
	// nested booleans under enabledPlugins.
	writeTestJSON(t, target, map[string]interface{}{
		"enabledPlugins": map[string]interface{}{
			"user-plugin@marketplace": true,
		},
	})

	entries, err := inst.Preview(adapter, home, project)
	if err != nil {
		t.Fatalf("Preview() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", entries[0].Action, domain.ActionUpdate)
	}
	if !strings.Contains(entries[0].Diff, "test@official") {
		t.Errorf("diff should add test@official, got:\n%s", entries[0].Diff)
	}
	if len(entries[0].Conflicts) != 0 {
		t.Errorf("plugins previewer should not report conflicts, got %v", entries[0].Conflicts)
	}
}

func TestPreview_SkillFile_WithoutProjectSkills_SkipEmptyTarget(t *testing.T) {
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

	entries, err := inst.Preview(adapter, home, project)
	if err != nil {
		t.Fatalf("Preview() error: %v", err)
	}
	// OpenCode exposes a ProjectSkillsDir, so ensure the non-empty path is used.
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].TargetPath == "" {
		t.Skip("adapter returned empty skills dir — planner emits skip with empty target, diff should also be empty")
	}
	expectedPath := filepath.Join(adapter.ProjectSkillsDir(project), "superpowers", "SKILL.md")
	if entries[0].TargetPath != expectedPath {
		t.Errorf("TargetPath = %q, want %q", entries[0].TargetPath, expectedPath)
	}
	if entries[0].Action != domain.ActionCreate {
		t.Errorf("Action = %q, want %q", entries[0].Action, domain.ActionCreate)
	}
	if !strings.Contains(entries[0].Diff, "Superpowers") {
		t.Errorf("diff should contain skill content, got:\n%s", entries[0].Diff)
	}
}
