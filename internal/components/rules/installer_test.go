package rules

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
	inst := New(domain.RulesConfig{}, "")
	if inst.ID() != domain.ComponentRules {
		t.Errorf("ID() = %q, want %q", inst.ID(), domain.ComponentRules)
	}
}

// ─── Content resolution ─────────────────────────────────────────────────────

func TestNew_InlineContent(t *testing.T) {
	cfg := domain.RulesConfig{
		TeamStandards: "Always use gofmt.",
	}
	inst := New(cfg, "/some/project")
	if inst.Content() != "Always use gofmt." {
		t.Errorf("Content() = %q, want %q", inst.Content(), "Always use gofmt.")
	}
}

func TestNew_FileContent(t *testing.T) {
	project := t.TempDir()
	amDir := filepath.Join(project, ".squadai")
	if err := os.MkdirAll(amDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(amDir, "standards.md"), []byte("File-based standards."), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := domain.RulesConfig{
		TeamStandardsFile: "standards.md",
	}
	inst := New(cfg, project)
	if inst.Content() != "File-based standards." {
		t.Errorf("Content() = %q, want %q", inst.Content(), "File-based standards.")
	}
}

func TestNew_InlineTakesPrecedenceOverFile(t *testing.T) {
	project := t.TempDir()
	amDir := filepath.Join(project, ".squadai")
	if err := os.MkdirAll(amDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(amDir, "standards.md"), []byte("file content"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := domain.RulesConfig{
		TeamStandards:     "inline content",
		TeamStandardsFile: "standards.md",
	}
	inst := New(cfg, project)
	if inst.Content() != "inline content" {
		t.Errorf("Content() = %q, want %q", inst.Content(), "inline content")
	}
}

func TestNew_EmptyConfig(t *testing.T) {
	inst := New(domain.RulesConfig{}, "/some/project")
	if inst.Content() != "" {
		t.Errorf("Content() = %q, want empty", inst.Content())
	}
}

func TestNew_MissingFile_ReturnsEmpty(t *testing.T) {
	cfg := domain.RulesConfig{
		TeamStandardsFile: "nonexistent.md",
	}
	inst := New(cfg, t.TempDir())
	if inst.Content() != "" {
		t.Errorf("Content() = %q, want empty for missing file", inst.Content())
	}
}

func TestNew_EmptyProjectDir_FileRef_ReturnsEmpty(t *testing.T) {
	cfg := domain.RulesConfig{
		TeamStandardsFile: "standards.md",
	}
	inst := New(cfg, "")
	if inst.Content() != "" {
		t.Errorf("Content() = %q, want empty when projectDir is empty", inst.Content())
	}
}

// ─── Plan (OpenCode) ────────────────────────────────────────────────────────

func TestPlan_OpenCode_NewFile_ReturnsCreate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(domain.RulesConfig{TeamStandards: "Use gofmt."}, project)

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
	expected := filepath.Join(project, "AGENTS.md")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
	if actions[0].Component != domain.ComponentRules {
		t.Errorf("Component = %q, want %q", actions[0].Component, domain.ComponentRules)
	}
}

func TestPlan_OpenCode_ExistingFileNoSection_ReturnsUpdate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(domain.RulesConfig{TeamStandards: "Use gofmt."}, project)

	targetPath := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(targetPath, []byte("# Existing Prompt\n"), 0644); err != nil {
		t.Fatal(err)
	}

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

func TestPlan_OpenCode_UpToDate_ReturnsSkip(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	content := "Use gofmt."
	inst := New(domain.RulesConfig{TeamStandards: content}, project)

	targetPath := filepath.Join(project, "AGENTS.md")
	fileContent := marker.InjectSection("", SectionID, content)
	if err := os.WriteFile(targetPath, []byte(fileContent), 0644); err != nil {
		t.Fatal(err)
	}

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

func TestPlan_OpenCode_OutdatedSection_ReturnsUpdate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(domain.RulesConfig{TeamStandards: "New standards."}, project)

	targetPath := filepath.Join(project, "AGENTS.md")
	fileContent := marker.InjectSection("", SectionID, "Old standards.")
	if err := os.WriteFile(targetPath, []byte(fileContent), 0644); err != nil {
		t.Fatal(err)
	}

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

// ─── Plan (Claude) ──────────────────────────────────────────────────────────

func TestPlan_Claude_TargetsProjectLevel(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	inst := New(domain.RulesConfig{TeamStandards: "Standards."}, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	expected := filepath.Join(project, "CLAUDE.md")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

// ─── Plan (empty content) ───────────────────────────────────────────────────

func TestPlan_EmptyContent_ReturnsNil(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(domain.RulesConfig{}, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for empty content, got %d", len(actions))
	}
}

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_OpenCode_CreatesFileWithRulesSection(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	standards := "Always use gofmt.\nAlways run tests."
	inst := New(domain.RulesConfig{TeamStandards: standards}, project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, _ := os.ReadFile(actions[0].TargetPath)
	s := string(data)
	if !marker.HasSection(s, SectionID) {
		t.Error("expected team-standards marker section in file after apply")
	}
	if !strings.Contains(s, "Always use gofmt.") {
		t.Error("content should include team standards")
	}
}

func TestApply_Claude_CreatesProjectLevelFile(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	inst := New(domain.RulesConfig{TeamStandards: "Claude standards."}, project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, _ := os.ReadFile(actions[0].TargetPath)
	s := string(data)
	if !marker.HasSection(s, SectionID) {
		t.Error("expected team-standards marker section in file")
	}
	if !strings.Contains(s, "Claude standards.") {
		t.Error("content should include team standards")
	}
}

func TestApply_PreservesExistingContent(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(domain.RulesConfig{TeamStandards: "Standards."}, project)

	targetPath := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(targetPath, []byte("# My Custom Prompt\n\nDo not delete this.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(targetPath)
	s := string(data)
	if !strings.Contains(s, "# My Custom Prompt") {
		t.Error("existing content should be preserved")
	}
	if !strings.Contains(s, "Do not delete this.") {
		t.Error("existing content should be preserved")
	}
	if !marker.HasSection(s, SectionID) {
		t.Error("rules section should be injected")
	}
}

func TestApply_SkipDoesNothing(t *testing.T) {
	inst := New(domain.RulesConfig{}, "")
	err := inst.Apply(domain.PlannedAction{Action: domain.ActionSkip})
	if err != nil {
		t.Fatalf("Skip should succeed, got: %v", err)
	}
}

func TestApply_Idempotent(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	content := "Stable standards."
	inst := New(domain.RulesConfig{TeamStandards: content}, project)

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

func TestApply_UpdatesOutdatedSection(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()

	// Write old standards.
	targetPath := filepath.Join(project, "AGENTS.md")
	oldContent := marker.InjectSection("# Header\n", SectionID, "Old standards.")
	if err := os.WriteFile(targetPath, []byte(oldContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Install new standards.
	inst := New(domain.RulesConfig{TeamStandards: "New standards."}, project)
	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions[0].Action != domain.ActionUpdate {
		t.Fatalf("expected Update, got %q", actions[0].Action)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(targetPath)
	s := string(data)
	if !strings.Contains(s, "New standards.") {
		t.Error("should contain new standards")
	}
	if strings.Contains(s, "Old standards.") {
		t.Error("should not contain old standards")
	}
	if !strings.Contains(s, "# Header") {
		t.Error("should preserve header content")
	}
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestVerify_OpenCode_AllPass_AfterApply(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(domain.RulesConfig{TeamStandards: "Standards."}, project)

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
}

func TestVerify_Claude_AllPass_AfterApply(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	inst := New(domain.RulesConfig{TeamStandards: "Standards."}, project)

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
}

func TestVerify_FailsWhenFileIsMissing(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(domain.RulesConfig{TeamStandards: "Standards."}, project)

	results, err := inst.Verify(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("expected rules-file-exists check to fail")
	}
}

func TestVerify_FailsWhenMarkersAreMissing(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(domain.RulesConfig{TeamStandards: "Standards."}, project)

	targetPath := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(targetPath, []byte("no markers here"), 0644); err != nil {
		t.Fatal(err)
	}

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	foundMarkerCheck := false
	for _, r := range results {
		if r.Check == "rules-markers-present" {
			foundMarkerCheck = true
			if r.Passed {
				t.Error("markers-present should fail when no markers")
			}
		}
	}
	if !foundMarkerCheck {
		t.Error("expected rules-markers-present check in results")
	}
}

func TestVerify_FailsWhenContentOutdated(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(domain.RulesConfig{TeamStandards: "New standards."}, project)

	targetPath := filepath.Join(project, "AGENTS.md")
	content := marker.InjectSection("", SectionID, "Old standards.")
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	foundContentCheck := false
	for _, r := range results {
		if r.Check == "rules-content-current" {
			foundContentCheck = true
			if r.Passed {
				t.Error("content-current should fail when content is outdated")
			}
		}
	}
	if !foundContentCheck {
		t.Error("expected rules-content-current check in results")
	}
}

func TestVerify_EmptyContent_ReturnsNil(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(domain.RulesConfig{}, project)

	results, err := inst.Verify(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty content, got %d", len(results))
	}
}
