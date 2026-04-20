package permissions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// Compile-time assertion that Installer implements domain.Previewer.
var _ domain.Previewer = (*Installer)(nil)

func TestPreview_Claude_FreshInstall_CreateEntryWithDiffAndNoConflicts(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := New()

	entries, err := inst.Preview(claude.New(), home, project)
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
	if e.Component != domain.ComponentPermissions {
		t.Errorf("Component = %q, want %q", e.Component, domain.ComponentPermissions)
	}
	if !strings.Contains(e.Diff, "permissions") || !strings.Contains(e.Diff, metaKey) {
		t.Errorf("diff should reference the new permissions/meta keys, got:\n%s", e.Diff)
	}
	if len(e.Conflicts) != 0 {
		t.Errorf("permissions previewer never reports conflicts, got %v", e.Conflicts)
	}
}

func TestPreview_Claude_MarkerPresent_SkipWithEmptyDiff(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := New()

	target := filepath.Join(project, ".claude", "settings.json")
	writeJSON(t, target, map[string]interface{}{
		metaKey: metaValue,
		"permissions": map[string]interface{}{
			"deny": []interface{}{"Read(./.env*)"},
		},
	})

	entries, err := inst.Preview(claude.New(), home, project)
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

func TestPreview_Claude_UserFileWithoutMarker_UpdateMergesOverlay(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := New()

	target := filepath.Join(project, ".claude", "settings.json")
	// User has their own permissions block and no SquadAI marker. Preview
	// should produce an update diff and no conflicts (the permissions
	// installer's Apply is deep-merge, so the user entries are preserved).
	writeJSON(t, target, map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": []interface{}{"Bash(npm *)"},
		},
	})

	entries, err := inst.Preview(claude.New(), home, project)
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
		t.Error("update should produce a non-empty diff")
	}
	if len(entries[0].Conflicts) != 0 {
		t.Errorf("permissions overlay uses deep-merge, no conflicts expected, got %v", entries[0].Conflicts)
	}
}

func writeJSON(t *testing.T, path string, data map[string]interface{}) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
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
