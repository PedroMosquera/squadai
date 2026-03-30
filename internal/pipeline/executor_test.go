package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexmosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/alexmosquera/agent-manager-pro/internal/components/copilot"
	"github.com/alexmosquera/agent-manager-pro/internal/components/memory"
	"github.com/alexmosquera/agent-manager-pro/internal/domain"
	"github.com/alexmosquera/agent-manager-pro/internal/marker"
	"github.com/alexmosquera/agent-manager-pro/internal/planner"
)

func TestExecute_AllActionsSucceed(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	cfg := fullConfig()
	p := planner.New()
	actions, err := p.Plan(cfg, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	exec := New(
		p.ComponentInstallers(),
		p.CopilotManager(),
		project,
		cfg.Copilot.InstructionsTemplate,
	)

	report := exec.Execute(actions)

	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("expected all steps to succeed")
	}

	// Verify files were created.
	promptPath := adapter.SystemPromptFile(home)
	if _, err := os.Stat(promptPath); err != nil {
		t.Errorf("memory prompt file not created: %v", err)
	}
	copilotPath := filepath.Join(project, copilot.CopilotInstructionsPath)
	if _, err := os.Stat(copilotPath); err != nil {
		t.Errorf("copilot instructions not created: %v", err)
	}
}

func TestExecute_SkipActionsSucceed(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	// Pre-create everything so plan returns all skips.
	promptPath := adapter.SystemPromptFile(home)
	os.MkdirAll(filepath.Dir(promptPath), 0755)
	memContent := marker.InjectSection("", "memory", memory.ProtocolTemplate())
	os.WriteFile(promptPath, []byte(memContent), 0644)

	copilotPath := filepath.Join(project, copilot.CopilotInstructionsPath)
	os.MkdirAll(filepath.Dir(copilotPath), 0755)
	copilotContent := marker.InjectSection("", copilot.SectionID, copilot.TemplateContent("standard"))
	os.WriteFile(copilotPath, []byte(copilotContent), 0644)

	cfg := fullConfig()
	p := planner.New()
	actions, _ := p.Plan(cfg, []domain.Adapter{adapter}, home, project)

	exec := New(
		p.ComponentInstallers(),
		p.CopilotManager(),
		project,
		cfg.Copilot.InstructionsTemplate,
	)

	report := exec.Execute(actions)

	if !report.Success {
		t.Fatal("skip actions should succeed")
	}
	for _, s := range report.Steps {
		if s.Action.Action != domain.ActionSkip {
			t.Errorf("expected all actions to be skip, got %q for %q", s.Action.Action, s.Action.ID)
		}
	}
}

func TestExecute_UnknownComponent_RecordsFailure(t *testing.T) {
	project := t.TempDir()

	actions := []domain.PlannedAction{
		{
			ID:        "unknown-action",
			Component: domain.ComponentID("nonexistent"),
			Action:    domain.ActionCreate,
		},
	}

	exec := New(
		map[domain.ComponentID]domain.ComponentInstaller{},
		copilot.New(),
		project,
		"standard",
	)

	report := exec.Execute(actions)

	if report.Success {
		t.Fatal("should fail for unknown component")
	}
	if len(report.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(report.Steps))
	}
	if report.Steps[0].Status != domain.StepFailed {
		t.Errorf("step status = %q, want failed", report.Steps[0].Status)
	}
}

func TestExecute_ContinuesAfterFailure(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	cfg := fullConfig()
	p := planner.New()
	actions, _ := p.Plan(cfg, []domain.Adapter{adapter}, home, project)

	// Prepend a broken action — executor should continue with the rest.
	broken := domain.PlannedAction{
		ID:        "broken",
		Component: domain.ComponentID("ghost"),
		Action:    domain.ActionCreate,
	}
	allActions := append([]domain.PlannedAction{broken}, actions...)

	exec := New(
		p.ComponentInstallers(),
		p.CopilotManager(),
		project,
		cfg.Copilot.InstructionsTemplate,
	)

	report := exec.Execute(allActions)

	if report.Success {
		t.Fatal("report should not be successful when a step fails")
	}

	// The broken step should fail, but the rest should succeed.
	if report.Steps[0].Status != domain.StepFailed {
		t.Error("first step should fail")
	}

	successCount := 0
	for _, s := range report.Steps[1:] {
		if s.Status == domain.StepSuccess {
			successCount++
		}
	}
	if successCount == 0 {
		t.Error("remaining steps should have succeeded")
	}
}

func TestExecute_EmptyPlan(t *testing.T) {
	exec := New(nil, copilot.New(), t.TempDir(), "standard")
	report := exec.Execute(nil)

	if !report.Success {
		t.Error("empty plan should succeed")
	}
	if len(report.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(report.Steps))
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func fullConfig() *domain.MergedConfig {
	return &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Copilot: domain.CopilotConfig{
			InstructionsTemplate: "standard",
		},
	}
}
