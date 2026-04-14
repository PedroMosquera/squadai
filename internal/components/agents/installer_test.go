package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/cursor"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/adapters/windsurf"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// ─── Interface compliance ───────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	inst := New(nil, nil, "")
	if inst.ID() != domain.ComponentAgents {
		t.Errorf("ID() = %q, want %q", inst.ID(), domain.ComponentAgents)
	}
}

// ─── renderAgent ────────────────────────────────────────────────────────────

func TestRenderAgent_Full(t *testing.T) {
	def := domain.AgentDef{
		Description: "Code reviewer",
		Mode:        "subagent",
		Model:       "anthropic/claude-sonnet-4-5",
		Prompt:      "Review code for security issues.",
		Permission:  map[string]string{"edit": "deny", "bash": "deny"},
	}
	content := renderAgent("reviewer", def)
	if !strings.Contains(content, "description: Code reviewer") {
		t.Error("should contain description")
	}
	if !strings.Contains(content, "mode: subagent") {
		t.Error("should contain mode")
	}
	if !strings.Contains(content, "model: anthropic/claude-sonnet-4-5") {
		t.Error("should contain model")
	}
	if !strings.Contains(content, "permission:") {
		t.Error("should contain permission block")
	}
	if !strings.Contains(content, "  bash: deny") {
		t.Error("should contain bash permission")
	}
	if !strings.Contains(content, "Review code for security issues.") {
		t.Error("should contain prompt")
	}
	if !strings.HasPrefix(content, "---\n") {
		t.Error("should start with frontmatter")
	}
}

func TestRenderAgent_Minimal(t *testing.T) {
	def := domain.AgentDef{
		Description: "Helper",
	}
	content := renderAgent("helper", def)
	if !strings.Contains(content, "description: Helper") {
		t.Error("should contain description")
	}
	if strings.Contains(content, "mode:") {
		t.Error("should not contain mode when empty")
	}
	if strings.Contains(content, "model:") {
		t.Error("should not contain model when empty")
	}
}

// ─── Plan (OpenCode) ────────────────────────────────────────────────────────

func TestPlan_OpenCode_NewAgents_ReturnsCreate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testAgents(), nil, project)

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
	expected := filepath.Join(project, ".opencode", "agents", "reviewer.md")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestPlan_OpenCode_UpToDate_ReturnsSkip(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	agentDefs := testAgents()
	inst := New(agentDefs, nil, project)

	// Write the expected file.
	targetPath := filepath.Join(project, ".opencode", "agents", "reviewer.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatal(err)
	}
	content := renderAgent("reviewer", agentDefs["reviewer"])
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions[0].Action != domain.ActionSkip {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionSkip)
	}
}

func TestPlan_OpenCode_Outdated_ReturnsUpdate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testAgents(), nil, project)

	targetPath := filepath.Join(project, ".opencode", "agents", "reviewer.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionUpdate)
	}
}

// ─── Plan (unsupported adapters) ────────────────────────────────────────────

func TestPlan_Claude_NoTeam_ReturnsNil(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	// Claude does not support ComponentAgents (no ProjectAgentsDir), so custom
	// agents return nil. With no team config, this is the expected empty result.
	inst := New(testAgents(), nil, project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for claude without team, got %d", len(actions))
	}
}

func TestPlan_NoAgents_ReturnsEmpty(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(nil, nil, project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_CreatesAgentFile(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testAgents(), nil, project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, err := os.ReadFile(actions[0].TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "description: Code reviewer") {
		t.Error("should contain agent description")
	}
	if !strings.Contains(string(data), "Review code carefully.") {
		t.Error("should contain agent prompt")
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
	inst := New(testAgents(), nil, project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(actions[0].TargetPath)

	actions2, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions2[0].Action != domain.ActionSkip {
		t.Fatalf("second plan should be Skip, got %q", actions2[0].Action)
	}

	second, _ := os.ReadFile(actions[0].TargetPath)
	if string(first) != string(second) {
		t.Error("content should not change")
	}
}

func TestApply_Delete(t *testing.T) {
	project := t.TempDir()
	targetPath := filepath.Join(project, ".opencode", "agents", "old.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("old agent"), 0644); err != nil {
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

	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestVerify_AllPass_AfterApply(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testAgents(), nil, project)

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

func TestVerify_FailsWhenFileMissing(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testAgents(), nil, project)

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("should fail when agent file is missing")
	}
}

func TestVerify_NoAgents_ReturnsNil(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(nil, nil, project)

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// ─── Prompt from file ───────────────────────────────────────────────────────

func TestNew_ResolvesPromptFile(t *testing.T) {
	project := t.TempDir()
	amDir := filepath.Join(project, ".squadai")
	if err := os.MkdirAll(amDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(amDir, "reviewer-prompt.md"), []byte("File-based prompt."), 0644); err != nil {
		t.Fatal(err)
	}

	agents := map[string]domain.AgentDef{
		"reviewer": {
			Description: "Reviewer",
			Mode:        "subagent",
			PromptFile:  "reviewer-prompt.md",
		},
	}
	inst := New(agents, nil, project)

	// The prompt should be resolved from file.
	adapter := opencode.New()
	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(actions[0].TargetPath)
	if !strings.Contains(string(data), "File-based prompt.") {
		t.Error("should contain file-based prompt content")
	}
}

// ─── Custom agents backward compatibility ──────────────────────────────────

func TestCustomAgents_StillWork(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	// nil config — V1 behavior
	inst := New(testAgents(), nil, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 custom agent action, got %d", len(actions))
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}
	data, _ := os.ReadFile(actions[0].TargetPath)
	if !strings.Contains(string(data), "Code reviewer") {
		t.Error("custom agent content should be present")
	}
}

// ─── Team: No team / no methodology ────────────────────────────────────────

func TestPlanTeam_NoTeam_NoTeamActions(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := &domain.MergedConfig{
		Methodology: domain.MethodologyTDD,
		Team:        nil, // no team
	}
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions with nil team, got %d", len(actions))
	}
}

func TestPlanTeam_NoMethodology_NoTeamActions(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := &domain.MergedConfig{
		Team: domain.DefaultTeam(domain.MethodologyTDD),
		// no Methodology set
	}
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions with no methodology, got %d", len(actions))
	}
}

// ─── Team: Native delegation (OpenCode) ────────────────────────────────────

func TestPlanTeamNative_TDD_PlansOrchestratorAndFiveSubAgents(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := tddTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// TDD: orchestrator + brainstormer + planner + implementer + reviewer + debugger = 6
	if len(actions) != 6 {
		t.Errorf("expected 6 actions for TDD native, got %d", len(actions))
	}
}

func TestPlanTeamNative_SDD_PlansEightActions(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := sddTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// SDD: orchestrator + explorer + proposer + spec-writer + designer + task-planner + implementer + verifier = 8
	if len(actions) != 8 {
		t.Errorf("expected 8 actions for SDD native, got %d", len(actions))
	}
}

func TestPlanTeamNative_Conventional_PlansFourActions(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := conventionalTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Conventional: orchestrator + implementer + reviewer + tester = 4
	if len(actions) != 4 {
		t.Errorf("expected 4 actions for Conventional native, got %d", len(actions))
	}
}

func TestPlanTeamNative_OrchestratorPath(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := tddTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, a := range actions {
		if strings.HasSuffix(a.TargetPath, "/orchestrator.md") {
			found = true
			expected := filepath.Join(project, ".opencode", "agents", "orchestrator.md")
			if a.TargetPath != expected {
				t.Errorf("orchestrator path = %q, want %q", a.TargetPath, expected)
			}
			break
		}
	}
	if !found {
		t.Error("orchestrator.md action not found")
	}
}

func TestPlanTeamNative_SubAgentPath(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := tddTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, a := range actions {
		if strings.HasSuffix(a.TargetPath, "/brainstormer.md") {
			found = true
			expected := filepath.Join(project, ".opencode", "agents", "brainstormer.md")
			if a.TargetPath != expected {
				t.Errorf("brainstormer path = %q, want %q", a.TargetPath, expected)
			}
			break
		}
	}
	if !found {
		t.Error("brainstormer.md action not found")
	}
}

func TestPlanTeamNative_ExistingUpToDate_Skip(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := conventionalTeamConfig()
	inst := New(nil, cfg, project)

	// Pre-apply so files exist.
	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("Apply failed: %v", err)
		}
	}

	// Second plan should all be Skip.
	actions2, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("second plan error: %v", err)
	}
	for _, a := range actions2 {
		if a.Action != domain.ActionSkip {
			t.Errorf("action %q should be Skip after apply, got %q", a.TargetPath, a.Action)
		}
	}
}

func TestPlanTeamNative_ExistingOutdated_Update(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := tddTeamConfig()
	inst := New(nil, cfg, project)

	// Write stale orchestrator.
	agentsDir := filepath.Join(project, ".opencode", "agents")
	if err := os.MkdirAll(agentsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "orchestrator.md"), []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	for _, a := range actions {
		if strings.HasSuffix(a.TargetPath, "orchestrator.md") {
			if a.Action != domain.ActionUpdate {
				t.Errorf("stale orchestrator should be Update, got %q", a.Action)
			}
			return
		}
	}
	t.Error("orchestrator action not found")
}

func TestApplyTeamNative_WritesRenderedContent(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := tddTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}

	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("Apply(%q) failed: %v", a.TargetPath, err)
		}
	}

	// Orchestrator file should exist and contain TDD-specific content.
	data, err := os.ReadFile(filepath.Join(project, ".opencode", "agents", "orchestrator.md"))
	if err != nil {
		t.Fatalf("orchestrator.md not found: %v", err)
	}
	if !strings.Contains(string(data), "TDD") && !strings.Contains(string(data), "tdd") {
		t.Error("orchestrator content should mention TDD")
	}
}

func TestApplyTeamNative_TemplateVariables_Rendered(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := tddTeamConfig()
	cfg.Meta.Language = "Go"
	cfg.Meta.TestCommand = "go test ./..."
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}

	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("Apply failed: %v", err)
		}
	}

	data, _ := os.ReadFile(filepath.Join(project, ".opencode", "agents", "orchestrator.md"))
	content := string(data)

	// Template variables should be rendered — no raw {{.Language}} or {{.TestCommand}}
	if strings.Contains(content, "{{.Language}}") {
		t.Error("{{.Language}} was not rendered")
	}
	if strings.Contains(content, "{{.TestCommand}}") {
		t.Error("{{.TestCommand}} was not rendered")
	}
	// The actual values should appear (template had conditionals so check both rendered and not-present)
	if !strings.Contains(content, "Go") {
		t.Error("Language 'Go' should appear in rendered content")
	}
}

// ─── Team: Cursor (also native) ────────────────────────────────────────────

func TestPlanTeamNative_Cursor_TDD(t *testing.T) {
	project := t.TempDir()
	adapter := cursor.New()
	cfg := tddTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 6 {
		t.Errorf("expected 6 actions for Cursor TDD, got %d", len(actions))
	}
	// Verify paths are in .cursor/agents/
	for _, a := range actions {
		if !strings.Contains(a.TargetPath, ".cursor/agents") {
			t.Errorf("Cursor path should contain .cursor/agents, got %q", a.TargetPath)
		}
	}
}

// ─── Team: Prompt delegation (Claude Code) ─────────────────────────────────

func TestPlanTeamPrompt_TDD_SingleAction(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	cfg := tddTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action for Claude Code TDD, got %d", len(actions))
	}
}

func TestPlanTeamPrompt_TargetIsRulesFile(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	cfg := tddTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := adapter.ProjectRulesFile(project) // CLAUDE.md
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestApplyTeamPrompt_InjectsMarkerBlock(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	cfg := tddTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, err := os.ReadFile(actions[0].TargetPath)
	if err != nil {
		t.Fatalf("rules file not found: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "<!-- squadai:team -->") {
		t.Error("should contain opening team marker")
	}
	if !strings.Contains(content, "<!-- /squadai:team -->") {
		t.Error("should contain closing team marker")
	}
}

func TestApplyTeamPrompt_PreservesExistingContent(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	cfg := tddTeamConfig()
	inst := New(nil, cfg, project)

	// Write existing content to the rules file.
	rulesPath := adapter.ProjectRulesFile(project)
	if err := os.MkdirAll(filepath.Dir(rulesPath), 0755); err != nil {
		t.Fatal(err)
	}
	existingContent := "# Existing Content\n\nSome user rules here.\n"
	if err := os.WriteFile(rulesPath, []byte(existingContent), 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, _ := os.ReadFile(rulesPath)
	content := string(data)
	if !strings.Contains(content, "# Existing Content") {
		t.Error("should preserve existing content")
	}
	if !strings.Contains(content, "Some user rules here.") {
		t.Error("should preserve user rules")
	}
	if !marker.HasSection(content, "team") {
		t.Error("should have team marker section")
	}
}

// ─── Team: Solo delegation (VS Code, Windsurf) ─────────────────────────────

func TestPlanTeamSolo_TDD_SingleAction_VSCode(t *testing.T) {
	project := t.TempDir()
	adapter := vscode.New()
	cfg := tddTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action for VSCode TDD solo, got %d", len(actions))
	}
}

func TestPlanTeamSolo_TargetIsRulesFile(t *testing.T) {
	project := t.TempDir()
	adapter := windsurf.New()
	cfg := tddTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := adapter.ProjectRulesFile(project) // .windsurfrules
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestApplyTeamSolo_InjectsMarkerBlock(t *testing.T) {
	project := t.TempDir()
	adapter := windsurf.New()
	cfg := sddTeamConfig()
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, err := os.ReadFile(actions[0].TargetPath)
	if err != nil {
		t.Fatalf("rules file not found: %v", err)
	}
	if !marker.HasSection(string(data), "team") {
		t.Error("should have team marker section in .windsurfrules")
	}
}

// ─── Team: Verify ───────────────────────────────────────────────────────────

func TestVerifyTeamNative_AfterApply_AllPass(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := tddTeamConfig()
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

func TestVerifyTeamPrompt_AfterApply_AllPass(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	cfg := tddTeamConfig()
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

func testAgents() map[string]domain.AgentDef {
	return map[string]domain.AgentDef{
		"reviewer": {
			Description: "Code reviewer",
			Mode:        "subagent",
			Model:       "anthropic/claude-sonnet-4-5",
			Prompt:      "Review code carefully.",
			Permission:  map[string]string{"edit": "deny"},
		},
	}
}

func tddTeamConfig() *domain.MergedConfig {
	return &domain.MergedConfig{
		Methodology: domain.MethodologyTDD,
		Team:        domain.DefaultTeam(domain.MethodologyTDD),
		MCP:         map[string]domain.MCPServerDef{},
		Meta:        domain.ProjectMeta{},
	}
}

func sddTeamConfig() *domain.MergedConfig {
	return &domain.MergedConfig{
		Methodology: domain.MethodologySDD,
		Team:        domain.DefaultTeam(domain.MethodologySDD),
		MCP:         map[string]domain.MCPServerDef{},
		Meta:        domain.ProjectMeta{},
	}
}

func conventionalTeamConfig() *domain.MergedConfig {
	return &domain.MergedConfig{
		Methodology: domain.MethodologyConventional,
		Team:        domain.DefaultTeam(domain.MethodologyConventional),
		MCP:         map[string]domain.MCPServerDef{},
		Meta:        domain.ProjectMeta{},
	}
}

// ─── Template: language rendering ───────────────────────────────────────────

func TestOrchestratorTemplate_ContainsLanguage(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cfg := tddTeamConfig()
	cfg.Meta.Language = "Go"
	cfg.Meta.TestCommand = "go test ./..."
	inst := New(nil, cfg, project)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("plan error: %v", err)
	}

	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("Apply(%q) failed: %v", a.TargetPath, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(project, ".opencode", "agents", "orchestrator.md"))
	if err != nil {
		t.Fatalf("orchestrator.md not found: %v", err)
	}
	content := string(data)

	if !strings.Contains(content, "Go") {
		t.Error("rendered orchestrator should contain the language 'Go'")
	}
	// Template variables should be fully rendered — no raw Go template syntax.
	if strings.Contains(content, "{{.Language}}") {
		t.Error("{{.Language}} should be rendered, not left as raw template syntax")
	}
}
