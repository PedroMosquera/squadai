package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// ─── Interface compliance ───────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	inst := New()
	if inst.ID() != domain.ComponentMemory {
		t.Errorf("ID() = %q, want %q", inst.ID(), domain.ComponentMemory)
	}
}

// ─── Per-adapter template selection ─────────────────────────────────────────

func TestTemplateForAdapter_OpenCode(t *testing.T) {
	adapter := opencode.New()
	content := templateForAdapter(adapter)
	if !strings.Contains(content, "AGENTS.md") {
		t.Error("OpenCode template should reference AGENTS.md")
	}
	if !strings.Contains(content, ".squadai/") {
		t.Error("OpenCode template should reference .squadai/")
	}
}

func TestTemplateForAdapter_Claude(t *testing.T) {
	adapter := claude.New()
	content := templateForAdapter(adapter)
	if !strings.Contains(content, "CLAUDE.md") {
		t.Error("Claude template should reference CLAUDE.md")
	}
}

func TestTemplateForAgentID_Unknown(t *testing.T) {
	content := templateForAgentID("unknown-agent")
	if !strings.Contains(content, "Memory Protocol") {
		t.Error("unknown agent should get generic template")
	}
	if !strings.Contains(content, "persistent memory tools") {
		t.Error("generic template should mention persistent memory tools")
	}
}

func TestTemplateForAgentID_Exported(t *testing.T) {
	content := TemplateForAgentID(domain.AgentOpenCode)
	if !strings.Contains(content, "AGENTS.md") {
		t.Error("exported TemplateForAgentID should match internal for OpenCode")
	}
}

// ─── Project-level target path ──────────────────────────────────────────────

func TestMemoryTargetPath_OpenCode_UsesProjectLevel(t *testing.T) {
	adapter := opencode.New()
	home := "/Users/test"
	project := "/Users/test/myproject"

	target := memoryTargetPath(adapter, home, project)
	expected := filepath.Join(project, "AGENTS.md")
	if target != expected {
		t.Errorf("target = %q, want %q", target, expected)
	}
}

func TestMemoryTargetPath_Claude_UsesProjectLevel(t *testing.T) {
	adapter := claude.New()
	home := "/Users/test"
	project := "/Users/test/myproject"

	target := memoryTargetPath(adapter, home, project)
	expected := filepath.Join(project, "CLAUDE.md")
	if target != expected {
		t.Errorf("target = %q, want %q", target, expected)
	}
}

func TestMemoryTargetPath_EmptyProjectDir_UsesGlobal(t *testing.T) {
	adapter := opencode.New()
	home := "/Users/test"

	target := memoryTargetPath(adapter, home, "")
	expected := adapter.SystemPromptFile(home)
	if target != expected {
		t.Errorf("target = %q, want %q", target, expected)
	}
}

// ─── Plan (OpenCode, project-level) ─────────────────────────────────────────

func TestPlan_OpenCode_NewFile_ReturnsCreate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

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
	// Target should be project-level AGENTS.md.
	expected := filepath.Join(project, "AGENTS.md")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestPlan_OpenCode_ExistingFileNoSection_ReturnsUpdate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	// Create target with existing content but no memory section.
	targetPath := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(targetPath, []byte("# Existing Prompt\n\nSome content.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	actions, err := inst.Plan(adapter, home, project)
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

func TestPlan_OpenCode_UpToDate_ReturnsSkip(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	// Create file with correct OpenCode memory section.
	targetPath := filepath.Join(project, "AGENTS.md")
	content := marker.InjectSection("", SectionID, openCodeMemoryTemplate())
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	actions, err := inst.Plan(adapter, home, project)
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

func TestPlan_OpenCode_OutdatedSection_ReturnsUpdate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	targetPath := filepath.Join(project, "AGENTS.md")
	content := marker.InjectSection("", SectionID, "old protocol content")
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	actions, err := inst.Plan(adapter, home, project)
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

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_OpenCode_CreatesFileWithMemorySection(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	actions, _ := inst.Plan(adapter, home, project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	err := inst.Apply(actions[0])
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	content, _ := os.ReadFile(actions[0].TargetPath)
	if !marker.HasSection(string(content), SectionID) {
		t.Error("expected memory marker section in file after apply")
	}
	if !strings.Contains(string(content), "AGENTS.md") {
		t.Error("OpenCode memory content should reference AGENTS.md")
	}
}

func TestApply_Claude_CreatesProjectLevelFile(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := New()

	actions, _ := inst.Plan(adapter, home, project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	err := inst.Apply(actions[0])
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	content, _ := os.ReadFile(actions[0].TargetPath)
	if !marker.HasSection(string(content), SectionID) {
		t.Error("expected memory marker section in file after apply")
	}
	if !strings.Contains(string(content), "CLAUDE.md") {
		t.Error("Claude memory content should reference CLAUDE.md")
	}
}

func TestApply_PreservesExistingContent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	targetPath := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(targetPath, []byte("# My Custom Prompt\n\nDo not delete this.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, home, project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(targetPath)
	s := string(content)
	if !strings.Contains(s, "# My Custom Prompt") {
		t.Error("existing content should be preserved")
	}
	if !strings.Contains(s, "Do not delete this.") {
		t.Error("existing content should be preserved")
	}
	if !marker.HasSection(s, SectionID) {
		t.Error("memory section should be injected")
	}
}

func TestApply_SkipDoesNothing(t *testing.T) {
	inst := New()
	action := domain.PlannedAction{
		Action: domain.ActionSkip,
	}
	err := inst.Apply(action)
	if err != nil {
		t.Fatalf("Skip should succeed, got: %v", err)
	}
}

func TestApply_Idempotent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	// First apply.
	actions, _ := inst.Plan(adapter, home, project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	first, _ := os.ReadFile(actions[0].TargetPath)

	// Second plan — should be Skip.
	actions2, _ := inst.Plan(adapter, home, project)
	if actions2[0].Action != domain.ActionSkip {
		t.Fatalf("second plan should be Skip, got %q", actions2[0].Action)
	}

	second, _ := os.ReadFile(actions[0].TargetPath)
	if string(first) != string(second) {
		t.Error("content should not change on second apply")
	}
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestVerify_OpenCode_AllPass_AfterApply(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	actions, _ := inst.Plan(adapter, home, project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
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

func TestVerify_Claude_AllPass_AfterApply(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := New()

	actions, _ := inst.Plan(adapter, home, project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
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

func TestVerify_FailsWhenFileIsMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	results, err := inst.Verify(adapter, home, project)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("expected file-exists check to fail")
	}
}

func TestVerify_FailsWhenMarkersAreMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	targetPath := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(targetPath, []byte("no markers"), 0644); err != nil {
		t.Fatal(err)
	}

	results, _ := inst.Verify(adapter, home, project)

	foundMarkerCheck := false
	for _, r := range results {
		if r.Check == "memory-markers-present" {
			foundMarkerCheck = true
			if r.Passed {
				t.Error("markers-present should fail when no markers")
			}
		}
	}
	if !foundMarkerCheck {
		t.Error("expected memory-markers-present check in results")
	}
}

func TestVerify_FailsWhenContentOutdated(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	// Write file with wrong content in the marker.
	targetPath := filepath.Join(project, "AGENTS.md")
	content := marker.InjectSection("", SectionID, "wrong old content")
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results, _ := inst.Verify(adapter, home, project)
	foundContentCheck := false
	for _, r := range results {
		if r.Check == "memory-content-current" {
			foundContentCheck = true
			if r.Passed {
				t.Error("content-current should fail when content is outdated")
			}
		}
	}
	if !foundContentCheck {
		t.Error("expected memory-content-current check in results")
	}
}
