package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// Compile-time assertion that Installer implements domain.Previewer.
var _ domain.Previewer = (*Installer)(nil)

func TestPreview_OpenCode_FreshInstall_CreateEntryWithDiff(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := New()

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
	if e.Component != domain.ComponentMemory {
		t.Errorf("Component = %q, want %q", e.Component, domain.ComponentMemory)
	}
	if !strings.Contains(e.Diff, marker.OpenTag(SectionID)) {
		t.Errorf("diff should contain the memory open tag, got:\n%s", e.Diff)
	}
	if len(e.Conflicts) != 0 {
		t.Errorf("memory previewer never reports conflicts, got %v", e.Conflicts)
	}
}

func TestPreview_OpenCode_UpToDate_SkipWithEmptyDiff(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := New()

	target := filepath.Join(project, "AGENTS.md")
	content := marker.InjectSection("", SectionID, templateForAgentID(domain.AgentOpenCode))
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

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
}

func TestPreview_OpenCode_PreservesUserContentOutsideMarkers(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := New()

	target := filepath.Join(project, "AGENTS.md")
	userPreamble := "# My Project\n\nCustom instructions above the managed section.\n"
	if err := os.WriteFile(target, []byte(userPreamble), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
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
	// Diff must reference the new marker block but MUST NOT remove user lines.
	if !strings.Contains(entries[0].Diff, marker.OpenTag(SectionID)) {
		t.Errorf("diff should reference the memory marker, got:\n%s", entries[0].Diff)
	}
	if strings.Contains(entries[0].Diff, "-# My Project") {
		t.Errorf("diff should NOT remove user preamble, got:\n%s", entries[0].Diff)
	}
	if len(entries[0].Conflicts) != 0 {
		t.Errorf("memory previewer never reports conflicts, got %v", entries[0].Conflicts)
	}
}
