package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// ─── Interface compliance ───────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	inst := New(nil, nil, "")
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

// VS Code and Cursor require name to match the directory, and Pi's strict
// YAML parser rejects a plain description containing ": ".
func TestRenderSkill_NameAndQuotedDescription(t *testing.T) {
	content := renderSkill("deploy", domain.SkillDef{Description: "Deploy steps: build, push", Content: "Run it."})
	want := "---\nname: deploy\ndescription: \"Deploy steps: build, push\"\n---\n\nRun it.\n"
	if content != want {
		t.Errorf("renderSkill =\n%q\nwant\n%q", content, want)
	}
}

func TestRenderSkill_ContentWithFrontmatterIsNotWrappedAgain(t *testing.T) {
	own := "---\nname: testing\ndescription: Test writing protocol\n---\n\n# Testing\n"
	content := renderSkill("testing", domain.SkillDef{Description: "Ignored", Content: own})
	if content != own {
		t.Errorf("content that brings its own frontmatter must be written as is; got\n%s", content)
	}
}

// ─── Plan (OpenCode) — V1 backward compat ──────────────────────────────────

func TestPlan_OpenCode_NewSkills_ReturnsCreate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testSkills(), nil, project)

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
	expected := filepath.Join(project, ".agents", "skills", "deploy", "SKILL.md")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestPlan_OpenCode_UpToDate_ReturnsSkip(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	skillDefs := testSkills()
	inst := New(skillDefs, nil, project)

	targetPath := filepath.Join(project, ".agents", "skills", "deploy", "SKILL.md")
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
	inst := New(testSkills(), nil, project)

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
	inst := New(skillDefs, nil, project)

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
	inst := New(testSkills(), nil, project)

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
	inst := New(testSkills(), nil, project)

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
	inst := New(nil, nil, project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_CreatesSkillFile(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testSkills(), nil, project)

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
	inst := New(nil, nil, "")
	err := inst.Apply(domain.PlannedAction{Action: domain.ActionSkip})
	if err != nil {
		t.Fatalf("Skip should succeed, got: %v", err)
	}
}

func TestApply_Idempotent(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testSkills(), nil, project)

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

	inst := New(nil, nil, project)
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
	inst := New(testSkills(), nil, project)

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
	inst := New(testSkills(), nil, project)

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
	inst := New(nil, nil, project)

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// ─── Content from file ──────────────────────────────────────────────────────

func TestNew_ResolvesContentFile(t *testing.T) {
	project := t.TempDir()
	amDir := filepath.Join(project, ".squadai")
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
	inst := New(skills, nil, project)
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

// ─── Custom skills backward compatibility ──────────────────────────────────

func TestCustomSkills_StillWork(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testSkills(), nil, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 custom skill action, got %d", len(actions))
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	data, _ := os.ReadFile(actions[0].TargetPath)
	if !strings.Contains(string(data), "Docker deployment") {
		t.Error("custom skill content should be present")
	}
}

// ─── No methodology — no embedded skills ───────────────────────────────────

func TestNoMethodology_NoEmbeddedSkills(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := &domain.MergedConfig{} // no methodology
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions with no methodology, got %d", len(actions))
	}
}

// ─── Methodology skills: action counts ────────────────────────────────────

func TestPlanMethodologySkills_TDD_OpenCode_EightActions(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyTDD}
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 5 TDD skills + 6 shared = 11
	if len(actions) != 11 {
		t.Errorf("expected 11 actions for TDD (5+6), got %d", len(actions))
	}
}

func TestPlanMethodologySkills_SDD_TenActions(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := &domain.MergedConfig{Methodology: domain.MethodologySDD}
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 7 SDD skills + 6 shared = 13
	if len(actions) != 13 {
		t.Errorf("expected 13 actions for SDD (7+6), got %d", len(actions))
	}
}

func TestPlanMethodologySkills_Conventional_ThreeSharedOnly(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyConventional}
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Conventional: 0 methodology-specific + 6 shared = 6
	if len(actions) != 6 {
		t.Errorf("expected 6 actions for Conventional (shared only), got %d", len(actions))
	}
}

func TestPlanMethodologySkills_TDD_CorrectPaths(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyTDD}
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	skillsDir := filepath.Join(project, ".agents", "skills")
	expectedPaths := map[string]bool{
		filepath.Join(skillsDir, "code-review", "SKILL.md"):                 false,
		filepath.Join(skillsDir, "find-skills", "SKILL.md"):                 false,
		filepath.Join(skillsDir, "testing", "SKILL.md"):                     false,
		filepath.Join(skillsDir, "pr-description", "SKILL.md"):              false,
		filepath.Join(skillsDir, "brainstorming", "SKILL.md"):               false,
		filepath.Join(skillsDir, "writing-plans", "SKILL.md"):               false,
		filepath.Join(skillsDir, "test-driven-development", "SKILL.md"):     false,
		filepath.Join(skillsDir, "subagent-driven-development", "SKILL.md"): false,
		filepath.Join(skillsDir, "systematic-debugging", "SKILL.md"):        false,
	}

	for _, a := range actions {
		if _, ok := expectedPaths[a.TargetPath]; ok {
			expectedPaths[a.TargetPath] = true
		}
	}

	for path, found := range expectedPaths {
		if !found {
			t.Errorf("expected action for path %q not found", path)
		}
	}
}

func TestPlanMethodologySkills_ExistingUpToDate_Skip(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyConventional}
	inst := New(nil, cfg, project)

	// Apply first.
	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("Apply failed: %v", err)
		}
	}

	// Second plan should be all Skip.
	actions2, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("second plan error: %v", err)
	}
	for _, a := range actions2 {
		if a.Action != domain.ActionSkip {
			t.Errorf("action for %q should be Skip, got %q", a.TargetPath, a.Action)
		}
	}
}

func TestApplyMethodologySkills_WritesEmbeddedContent(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyConventional}
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("Apply(%q) failed: %v", a.TargetPath, err)
		}
	}

	// Verify a shared skill file was written with YAML frontmatter.
	codeReviewPath := filepath.Join(project, ".agents", "skills", "code-review", "SKILL.md")
	data, err := os.ReadFile(codeReviewPath)
	if err != nil {
		t.Fatalf("code-review skill not written: %v", err)
	}
	if !strings.Contains(string(data), "---") {
		t.Error("embedded skill should have YAML frontmatter")
	}
}

func TestVerifyMethodologySkills_AfterApply_AllPass(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyConventional}
	inst := New(nil, cfg, project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("Apply failed: %v", err)
		}
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

// ─── Helpers ────────────────────────────────────────────────────────────────

func TestSharedSkillPaths_IncludesFindSkills(t *testing.T) {
	paths := sharedSkillPaths()
	found := false
	for _, p := range paths {
		if p == "skills/shared/find-skills" {
			found = true
			break
		}
	}
	if !found {
		t.Error("sharedSkillPaths() must include skills/shared/find-skills")
	}
	if len(paths) != 6 {
		t.Errorf("expected 6 shared skill paths, got %d", len(paths))
	}
}

func testSkills() map[string]domain.SkillDef {
	return map[string]domain.SkillDef{
		"deploy": {
			Description: "Docker deployment",
			Content:     "Deploy using docker compose.",
		},
	}
}

func TestPlan_CustomSkillReplacesEmbeddedSkillOfTheSameName(t *testing.T) {
	project := t.TempDir()
	custom := map[string]domain.SkillDef{"testing": {Description: "Our testing rules", Content: "Use table tests."}}
	inst := New(custom, &domain.MergedConfig{Methodology: domain.MethodologyTDD}, project)

	actions, err := inst.Plan(opencode.New(), t.TempDir(), project)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(project, ".agents", "skills", "testing", "SKILL.md")
	var descs []string
	for _, a := range actions {
		if a.TargetPath == target {
			descs = append(descs, a.Description)
		}
	}
	if len(descs) != 1 || strings.HasPrefix(descs[0], "skill:embedded:") {
		t.Fatalf("actions for %s = %v, want only the custom skill", target, descs)
	}
}
