package workflows

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/claude"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/cursor"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/vscode"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/windsurf"
	"github.com/PedroMosquera/agent-manager-pro/internal/assets"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// ─── Interface compliance ───────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	inst := New(nil)
	if inst.ID() != domain.ComponentWorkflows {
		t.Errorf("ID() = %q, want %q", inst.ID(), domain.ComponentWorkflows)
	}
}

// ─── Plan: non-Windsurf adapters return nil ─────────────────────────────────

func TestPlan_OpenCode_ReturnsNil(t *testing.T) {
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyTDD}
	inst := New(cfg)
	adapter := opencode.New()

	actions, err := inst.Plan(adapter, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for OpenCode, got %d", len(actions))
	}
}

func TestPlan_Claude_ReturnsNil(t *testing.T) {
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyTDD}
	inst := New(cfg)
	adapter := claude.New()

	actions, _ := inst.Plan(adapter, t.TempDir(), t.TempDir())
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for Claude, got %d", len(actions))
	}
}

func TestPlan_VSCode_ReturnsNil(t *testing.T) {
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyTDD}
	inst := New(cfg)
	adapter := vscode.New()

	actions, _ := inst.Plan(adapter, t.TempDir(), t.TempDir())
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for VS Code, got %d", len(actions))
	}
}

func TestPlan_Cursor_ReturnsNil(t *testing.T) {
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyTDD}
	inst := New(cfg)
	adapter := cursor.New()

	actions, _ := inst.Plan(adapter, t.TempDir(), t.TempDir())
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for Cursor, got %d", len(actions))
	}
}

// ─── Plan: Windsurf with different methodologies ────────────────────────────

func TestPlan_Windsurf_TDD_PlansAction(t *testing.T) {
	project := t.TempDir()
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyTDD}
	inst := New(cfg)
	adapter := windsurf.New()

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
	expected := filepath.Join(project, ".windsurf", "workflows", "tdd-pipeline.md")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
	if actions[0].Component != domain.ComponentWorkflows {
		t.Errorf("Component = %q, want %q", actions[0].Component, domain.ComponentWorkflows)
	}
}

func TestPlan_Windsurf_SDD_PlansAction(t *testing.T) {
	project := t.TempDir()
	cfg := &domain.MergedConfig{Methodology: domain.MethodologySDD}
	inst := New(cfg)
	adapter := windsurf.New()

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	expected := filepath.Join(project, ".windsurf", "workflows", "sdd-pipeline.md")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestPlan_Windsurf_Conventional_PlansAction(t *testing.T) {
	project := t.TempDir()
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyConventional}
	inst := New(cfg)
	adapter := windsurf.New()

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	expected := filepath.Join(project, ".windsurf", "workflows", "conventional-pipeline.md")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestPlan_Windsurf_EmptyMethodology_ReturnsNil(t *testing.T) {
	cfg := &domain.MergedConfig{Methodology: ""}
	inst := New(cfg)
	adapter := windsurf.New()

	actions, err := inst.Plan(adapter, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for empty methodology, got %d", len(actions))
	}
}

func TestPlan_Windsurf_NilConfig_ReturnsNil(t *testing.T) {
	inst := New(nil)
	adapter := windsurf.New()

	actions, err := inst.Plan(adapter, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for nil config, got %d", len(actions))
	}
}

// ─── Idempotency ────────────────────────────────────────────────────────────

func TestPlan_Windsurf_Idempotent(t *testing.T) {
	project := t.TempDir()
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyTDD}
	inst := New(cfg)
	adapter := windsurf.New()

	// First plan + apply.
	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	// Second plan — should be Skip.
	actions2, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions2) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions2))
	}
	if actions2[0].Action != domain.ActionSkip {
		t.Errorf("second plan should be Skip, got %q", actions2[0].Action)
	}
}

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_CreatesDirectoryAndFile(t *testing.T) {
	project := t.TempDir()
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyTDD}
	inst := New(cfg)
	adapter := windsurf.New()

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify the directory was created.
	workflowsDir := filepath.Join(project, ".windsurf", "workflows")
	info, err := os.Stat(workflowsDir)
	if err != nil {
		t.Fatalf("workflows dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("workflows path should be a directory")
	}

	// Verify file content matches embedded asset.
	data, err := os.ReadFile(actions[0].TargetPath)
	if err != nil {
		t.Fatalf("workflow file not created: %v", err)
	}

	expected, _ := assets.Read("workflows/tdd-pipeline.md")
	if string(data) != expected {
		t.Error("workflow content does not match embedded asset")
	}
}

func TestApply_SkipDoesNothing(t *testing.T) {
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyTDD}
	inst := New(cfg)
	err := inst.Apply(domain.PlannedAction{Action: domain.ActionSkip})
	if err != nil {
		t.Fatalf("Skip should succeed, got: %v", err)
	}
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestVerify_PassesAfterApply(t *testing.T) {
	project := t.TempDir()
	cfg := &domain.MergedConfig{Methodology: domain.MethodologySDD}
	inst := New(cfg)
	adapter := windsurf.New()

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatal(err)
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
	if len(results) == 0 {
		t.Error("expected at least 1 verify result")
	}
}

func TestVerify_FailsWhenMissing(t *testing.T) {
	project := t.TempDir()
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyTDD}
	inst := New(cfg)
	adapter := windsurf.New()

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("should fail when workflow file missing")
	}
}

func TestVerify_NonWindsurf_ReturnsNil(t *testing.T) {
	cfg := &domain.MergedConfig{Methodology: domain.MethodologyTDD}
	inst := New(cfg)
	adapter := opencode.New()

	results, err := inst.Verify(adapter, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for non-Windsurf, got %d", len(results))
	}
}
