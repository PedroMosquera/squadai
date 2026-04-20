package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/windsurf"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// Compile-time assertion that Installer implements domain.Previewer.
var _ domain.Previewer = (*Installer)(nil)

const testStandards = "Write tests first. Keep functions small."

func newTestInstaller(t *testing.T) *Installer {
	t.Helper()
	inst, err := New(domain.RulesConfig{TeamStandards: testStandards}, t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return inst
}

func TestPreview_OpenCode_Marker_FreshInstall_CreateEntryWithDiff(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := newTestInstaller(t)

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
	if e.Component != domain.ComponentRules {
		t.Errorf("Component = %q, want %q", e.Component, domain.ComponentRules)
	}
	if !strings.Contains(e.Diff, marker.OpenTag(SectionID)) {
		t.Errorf("diff should contain the team-standards open tag, got:\n%s", e.Diff)
	}
	if !strings.Contains(e.Diff, "Write tests first") {
		t.Errorf("diff should contain the standards body, got:\n%s", e.Diff)
	}
	if len(e.Conflicts) != 0 {
		t.Errorf("rules previewer never reports conflicts, got %v", e.Conflicts)
	}
}

func TestPreview_OpenCode_Marker_UpToDate_SkipWithEmptyDiff(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := newTestInstaller(t)

	target := filepath.Join(project, "AGENTS.md")
	doc := marker.InjectSection("# Project\n\n", SectionID, testStandards)
	if err := os.WriteFile(target, []byte(doc), 0644); err != nil {
		t.Fatalf("seed: %v", err)
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

func TestPreview_OpenCode_Marker_PreservesUserContentOutsideBlock(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := newTestInstaller(t)

	target := filepath.Join(project, "AGENTS.md")
	userDoc := "# My Rules\n\nCustom user instructions.\n"
	if err := os.WriteFile(target, []byte(userDoc), 0644); err != nil {
		t.Fatalf("seed: %v", err)
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
	if strings.Contains(entries[0].Diff, "-# My Rules") {
		t.Errorf("diff should NOT remove user content, got:\n%s", entries[0].Diff)
	}
	if !strings.Contains(entries[0].Diff, marker.OpenTag(SectionID)) {
		t.Errorf("diff should add the team-standards marker block, got:\n%s", entries[0].Diff)
	}
}

func TestPreview_Windsurf_Frontmatter_FreshInstall_CreateEntryWithDiff(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	inst := newTestInstaller(t)

	entries, err := inst.Preview(windsurf.New(), home, project)
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
	if !strings.Contains(e.Diff, "trigger: always_on") {
		t.Errorf("diff should contain Windsurf frontmatter, got:\n%s", e.Diff)
	}
	if !strings.Contains(e.Diff, "Write tests first") {
		t.Errorf("diff should contain standards body, got:\n%s", e.Diff)
	}
}

func TestPreview_Windsurf_Frontmatter_UpToDate_SkipWithEmptyDiff(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	adapter := windsurf.New()
	inst := newTestInstaller(t)

	target := adapter.ProjectRulesFile(project)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := adapter.RulesFrontmatter() + testStandards + "\n"
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

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
