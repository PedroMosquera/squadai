package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/claude"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// ─── Interface compliance ───────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	inst := New(nil, "")
	if inst.ID() != domain.ComponentSkills {
		t.Errorf("ID() = %q, want %q", inst.ID(), domain.ComponentSkills)
	}
}

// ─── renderSkill ────────────────────────────────────────────────────────────

func TestRenderSkill(t *testing.T) {
	def := domain.SkillDef{
		Description: "Docker deployment",
		Content:     "Deploy using docker compose.",
	}
	content := renderSkill("deploy", def)
	if !strings.Contains(content, "description: Docker deployment") {
		t.Error("should contain description")
	}
	if !strings.Contains(content, "Deploy using docker compose.") {
		t.Error("should contain content")
	}
	if !strings.HasPrefix(content, "---\n") {
		t.Error("should start with frontmatter")
	}
}

// ─── Plan (OpenCode) ────────────────────────────────────────────────────────

func TestPlan_OpenCode_NewSkills_ReturnsCreate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testSkills(), project)

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
	expected := filepath.Join(project, ".opencode", "skills", "deploy", "SKILL.md")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestPlan_OpenCode_UpToDate_ReturnsSkip(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	skillDefs := testSkills()
	inst := New(skillDefs, project)

	targetPath := filepath.Join(project, ".opencode", "skills", "deploy", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatal(err)
	}
	content := renderSkill("deploy", skillDefs["deploy"])
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions[0].Action != domain.ActionSkip {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionSkip)
	}
}

func TestPlan_Claude_NewSkills_ReturnsCreate(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	inst := New(testSkills(), project)

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
	expected := filepath.Join(project, ".claude", "skills", "deploy", "SKILL.md")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestPlan_Claude_UpToDate_ReturnsSkip(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	skillDefs := testSkills()
	inst := New(skillDefs, project)

	targetPath := filepath.Join(project, ".claude", "skills", "deploy", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatal(err)
	}
	content := renderSkill("deploy", skillDefs["deploy"])
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions[0].Action != domain.ActionSkip {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionSkip)
	}
}

func TestApply_Claude_CreatesSkillFile(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	inst := New(testSkills(), project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, _ := os.ReadFile(actions[0].TargetPath)
	if !strings.Contains(string(data), "description: Docker deployment") {
		t.Error("should contain skill description")
	}
}

func TestVerify_Claude_AllPass_AfterApply(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	inst := New(testSkills(), project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
}

func TestPlan_NoSkills_ReturnsEmpty(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(nil, project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_CreatesSkillFile(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testSkills(), project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, _ := os.ReadFile(actions[0].TargetPath)
	if !strings.Contains(string(data), "description: Docker deployment") {
		t.Error("should contain skill description")
	}
}

func TestApply_SkipDoesNothing(t *testing.T) {
	inst := New(nil, "")
	err := inst.Apply(domain.PlannedAction{Action: domain.ActionSkip})
	if err != nil {
		t.Fatalf("Skip should succeed, got: %v", err)
	}
}

func TestApply_Idempotent(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testSkills(), project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	actions2, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions2[0].Action != domain.ActionSkip {
		t.Fatalf("second plan should be Skip, got %q", actions2[0].Action)
	}
}

func TestApply_Delete(t *testing.T) {
	project := t.TempDir()
	skillDir := filepath.Join(project, ".opencode", "skills", "old")
	targetPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	inst := New(nil, project)
	err := inst.Apply(domain.PlannedAction{
		Action:     domain.ActionDelete,
		TargetPath: targetPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Skill directory should be removed.
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Error("skill directory should be deleted")
	}
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestVerify_AllPass_AfterApply(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testSkills(), project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
}

func TestVerify_FailsWhenFileMissing(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testSkills(), project)

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("should fail when skill file is missing")
	}
}

func TestVerify_NoSkills_ReturnsNil(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(nil, project)

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// ─── Content from file ──────────────────────────────────────────────────────

func TestNew_ResolvesContentFile(t *testing.T) {
	project := t.TempDir()
	amDir := filepath.Join(project, ".agent-manager")
	if err := os.MkdirAll(amDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(amDir, "deploy-skill.md"), []byte("File-based skill."), 0644); err != nil {
		t.Fatal(err)
	}

	skills := map[string]domain.SkillDef{
		"deploy": {
			Description: "Deploy",
			ContentFile: "deploy-skill.md",
		},
	}
	inst := New(skills, project)
	adapter := opencode.New()
	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(actions[0].TargetPath)
	if !strings.Contains(string(data), "File-based skill.") {
		t.Error("should contain file-based content")
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func testSkills() map[string]domain.SkillDef {
	return map[string]domain.SkillDef{
		"deploy": {
			Description: "Docker deployment",
			Content:     "Deploy using docker compose.",
		},
	}
}
