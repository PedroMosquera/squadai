package mcp

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
	inst := newTestInstaller()

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
	if e.Component != domain.ComponentMCP {
		t.Errorf("Component = %q, want %q", e.Component, domain.ComponentMCP)
	}
	if !strings.Contains(e.Diff, "mcp") || !strings.Contains(e.Diff, "context7") {
		t.Errorf("expected diff to reference the new mcp/context7 keys, got:\n%s", e.Diff)
	}
	if len(e.Conflicts) != 0 {
		t.Errorf("fresh install should have no conflicts, got %v", e.Conflicts)
	}
}

func TestPreview_OpenCode_UpToDate_SkipWithEmptyDiff(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := newTestInstaller()

	target := filepath.Join(project, "opencode.json")
	writeTestJSON(t, target, map[string]interface{}{
		"mcp": map[string]interface{}{
			"context7": map[string]interface{}{
				"type": "remote",
				"url":  "https://mcp.context7.com/mcp",
			},
		},
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
	inst := newTestInstaller()

	target := filepath.Join(project, "opencode.json")
	writeTestJSON(t, target, map[string]interface{}{
		"mcp": map[string]interface{}{
			"context7": map[string]interface{}{
				"type": "remote",
				"url":  "https://stale.example.com/mcp",
			},
		},
	})

	// Record that SquadAI owns the "mcp" key for this file.
	relPath, err := filepath.Rel(project, target)
	if err != nil {
		t.Fatalf("rel: %v", err)
	}
	if err := managed.WriteManagedKeys(project, relPath, []string{"mcp"}); err != nil {
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
		t.Error("update of stale URL should produce a non-empty diff")
	}
	if len(entries[0].Conflicts) != 0 {
		t.Errorf("managed key overwrite must not produce conflicts, got %v", entries[0].Conflicts)
	}
}

func TestPreview_OpenCode_UserEditedMCPKey_EmitsConflict(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := newTestInstaller()

	target := filepath.Join(project, "opencode.json")
	// User has hand-edited the "mcp" key to a value SquadAI does not own
	// (no managed-keys sidecar). Preview should flag this as a conflict.
	writeTestJSON(t, target, map[string]interface{}{
		"mcp": map[string]interface{}{
			"user-added-server": map[string]interface{}{
				"type": "remote",
				"url":  "https://user.example.com/mcp",
			},
		},
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
	if c.Key != "mcp" {
		t.Errorf("Conflict.Key = %q, want %q", c.Key, "mcp")
	}
	if !strings.Contains(c.UserValue, "user-added-server") {
		t.Errorf("Conflict.UserValue should mention user-added-server, got %q", c.UserValue)
	}
	if !strings.Contains(c.IncomingValue, "context7") {
		t.Errorf("Conflict.IncomingValue should mention context7, got %q", c.IncomingValue)
	}
}
