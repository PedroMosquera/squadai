package settings

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/managed"
)

// Compile-time assertion that Installer implements domain.Previewer.
var _ domain.Previewer = (*Installer)(nil)

func TestPreview_OpenCode_FreshInstall_CreateEntryWithDiffAndNoConflicts(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := newTestInstaller("opencode", map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	})

	entries, err := inst.Preview(opencode.New(), home, project)
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
	if e.Component != domain.ComponentSettings {
		t.Errorf("Component = %q, want %q", e.Component, domain.ComponentSettings)
	}
	if !strings.Contains(e.Diff, "model") || !strings.Contains(e.Diff, "claude-sonnet-4-5") {
		t.Errorf("diff should reference the new model key, got:\n%s", e.Diff)
	}
	if len(e.Conflicts) != 0 {
		t.Errorf("fresh install should have no conflicts, got %v", e.Conflicts)
	}
}

func TestPreview_OpenCode_UpToDate_SkipWithEmptyDiff(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := newTestInstaller("opencode", map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	})

	target := filepath.Join(project, "opencode.json")
	writeTestJSON(t, target, map[string]interface{}{
		"$schema": "https://opencode.ai/config.json",
		"model":   "anthropic/claude-sonnet-4-5",
	})

	entries, err := inst.Preview(opencode.New(), home, project)
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
	if len(entries[0].Conflicts) != 0 {
		t.Errorf("skip entry should have no conflicts, got %v", entries[0].Conflicts)
	}
}

func TestPreview_OpenCode_ManagedOverwrite_NoConflict(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := newTestInstaller("opencode", map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	})

	target := filepath.Join(project, "opencode.json")
	writeTestJSON(t, target, map[string]interface{}{
		"model": "anthropic/claude-sonnet-3-5-stale",
	})
	if err := managed.WriteManagedKeys(project, "opencode.json", []string{"model"}); err != nil {
		t.Fatalf("seed managed keys: %v", err)
	}

	entries, err := inst.Preview(opencode.New(), home, project)
	if err != nil {
		t.Fatalf("Preview() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", entries[0].Action, domain.ActionUpdate)
	}
	if entries[0].Diff == "" {
		t.Error("update of stale model should produce a non-empty diff")
	}
	if len(entries[0].Conflicts) != 0 {
		t.Errorf("managed key overwrite must not produce conflicts, got %v", entries[0].Conflicts)
	}
}

func TestPreview_OpenCode_UserEditedModelKey_EmitsConflict(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := newTestInstaller("opencode", map[string]interface{}{
		"model": "anthropic/claude-sonnet-4-5",
	})

	target := filepath.Join(project, "opencode.json")
	// User hand-edited "model" — no managed-keys sidecar entry for it.
	// Preview should flag this as a conflict.
	writeTestJSON(t, target, map[string]interface{}{
		"model": "user/custom-model",
	})

	entries, err := inst.Preview(opencode.New(), home, project)
	if err != nil {
		t.Fatalf("Preview() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("want 1 entry, got %d", len(entries))
	}
	if entries[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", entries[0].Action, domain.ActionUpdate)
	}
	if len(entries[0].Conflicts) != 1 {
		t.Fatalf("expected 1 conflict, got %d: %v", len(entries[0].Conflicts), entries[0].Conflicts)
	}
	c := entries[0].Conflicts[0]
	if c.Key != "model" {
		t.Errorf("Conflict.Key = %q, want %q", c.Key, "model")
	}
	if !strings.Contains(c.UserValue, "user/custom-model") {
		t.Errorf("Conflict.UserValue should mention user/custom-model, got %q", c.UserValue)
	}
	if !strings.Contains(c.IncomingValue, "claude-sonnet-4-5") {
		t.Errorf("Conflict.IncomingValue should mention claude-sonnet-4-5, got %q", c.IncomingValue)
	}
}
