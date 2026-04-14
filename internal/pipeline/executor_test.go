package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/PedroMosquera/agent-manager-pro/internal/backup"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/copilot"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/memory"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/marker"
	"github.com/PedroMosquera/agent-manager-pro/internal/planner"
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
		cfg.Copilot,
		nil, // no backup store
	)

	report, err := exec.Execute(actions)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("expected all steps to succeed")
	}

	// Verify files were created.
	memoryPath := filepath.Join(project, "AGENTS.md") // OpenCode memory targets project-level
	if _, err := os.Stat(memoryPath); err != nil {
		t.Errorf("memory file not created: %v", err)
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
	memoryPath := filepath.Join(project, "AGENTS.md") // OpenCode memory targets project-level
	memContent := marker.InjectSection("", "memory", memory.TemplateForAgentID(domain.AgentOpenCode))
	if err := os.WriteFile(memoryPath, []byte(memContent), 0644); err != nil {
		t.Fatal(err)
	}

	copilotPath := filepath.Join(project, copilot.CopilotInstructionsPath)
	if err := os.MkdirAll(filepath.Dir(copilotPath), 0755); err != nil {
		t.Fatal(err)
	}
	copilotContent := marker.InjectSection("", copilot.SectionID, copilot.TemplateContent("standard"))
	if err := os.WriteFile(copilotPath, []byte(copilotContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := fullConfig()
	p := planner.New()
	actions, _ := p.Plan(cfg, []domain.Adapter{adapter}, home, project)

	exec := New(
		p.ComponentInstallers(),
		p.CopilotManager(),
		project,
		cfg.Copilot,
		nil,
	)

	report, err := exec.Execute(actions)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

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
		domain.CopilotConfig{InstructionsTemplate: "standard"},
		nil,
	)

	report, err := exec.Execute(actions)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

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

func TestExecute_ContinuesAfterFailure_NoBackup(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	cfg := fullConfig()
	p := planner.New()
	actions, _ := p.Plan(cfg, []domain.Adapter{adapter}, home, project)

	// Prepend a broken action — executor should continue with the rest (no backup).
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
		cfg.Copilot,
		nil, // no backup — legacy continue-on-failure behavior
	)

	report, err := exec.Execute(allActions)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

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
	exec := New(nil, copilot.New(), t.TempDir(), domain.CopilotConfig{InstructionsTemplate: "standard"}, nil)
	report, err := exec.Execute(nil)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !report.Success {
		t.Error("empty plan should succeed")
	}
	if len(report.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(report.Steps))
	}
}

func TestExecute_WithBackup_StopsOnFailureAndRollsBack(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	backupDir := t.TempDir()
	adapter := opencode.New()

	// Pre-create the memory file at project-level with known content.
	memoryPath := filepath.Join(project, "AGENTS.md")
	originalContent := "# Original user content\n"
	if err := os.WriteFile(memoryPath, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	store := backup.NewStore(backupDir)

	// Build a plan: first a real create action, then a broken action.
	cfg := fullConfig()
	p := planner.New()
	actions, _ := p.Plan(cfg, []domain.Adapter{adapter}, home, project)

	// Append a broken action after the real ones.
	broken := domain.PlannedAction{
		ID:          "broken",
		Component:   domain.ComponentID("ghost"),
		Action:      domain.ActionCreate,
		TargetPath:  filepath.Join(project, "ghost.txt"),
		Description: "broken action",
	}
	allActions := append(actions, broken)

	exec := New(
		p.ComponentInstallers(),
		p.CopilotManager(),
		project,
		cfg.Copilot,
		store,
	)

	report, err := exec.Execute(allActions)
	if err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	if report.Success {
		t.Fatal("report should not be successful")
	}

	if report.BackupID == "" {
		t.Fatal("backup ID should be set")
	}

	// The broken action should be the last executed one (failed).
	// Previous successful actions should have been executed, then rollback
	// restores the original files.
	hasRolledBack := false
	for _, s := range report.Steps {
		if s.Status == domain.StepRolledBack {
			hasRolledBack = true
		}
	}
	// The broken action is last in the list, so there are no remaining
	// actions to mark as rolled_back. But the files should be restored.
	_ = hasRolledBack

	// Verify the original file was restored.
	data, err := os.ReadFile(memoryPath)
	if err != nil {
		t.Fatalf("read prompt file: %v", err)
	}
	if string(data) != originalContent {
		t.Errorf("prompt file content = %q, want %q", string(data), originalContent)
	}

	// Verify manifest was updated.
	manifest, err := store.Get(report.BackupID)
	if err != nil {
		t.Fatalf("get manifest: %v", err)
	}
	if manifest.Status != "rolled_back" {
		t.Errorf("manifest status = %q, want rolled_back", manifest.Status)
	}
}

func TestExecute_WithBackup_FailureInMiddle_RemainingRolledBack(t *testing.T) {
	project := t.TempDir()
	backupDir := t.TempDir()

	store := backup.NewStore(backupDir)

	// Three actions: success, broken, pending.
	actions := []domain.PlannedAction{
		{
			ID:          "copilot-instructions",
			Action:      domain.ActionCreate,
			TargetPath:  filepath.Join(project, ".github", "copilot-instructions.md"),
			Description: "create copilot instructions",
		},
		{
			ID:          "broken",
			Component:   domain.ComponentID("ghost"),
			Action:      domain.ActionCreate,
			TargetPath:  filepath.Join(project, "ghost.txt"),
			Description: "broken action",
		},
		{
			ID:          "would-run",
			Component:   domain.ComponentID("another"),
			Action:      domain.ActionCreate,
			TargetPath:  filepath.Join(project, "another.txt"),
			Description: "this should not run",
		},
	}

	exec := New(
		map[domain.ComponentID]domain.ComponentInstaller{},
		copilot.New(),
		project,
		domain.CopilotConfig{InstructionsTemplate: "standard"},
		store,
	)

	report, err := exec.Execute(actions)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if report.Success {
		t.Fatal("should not succeed")
	}

	if len(report.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(report.Steps))
	}

	// First action: success (copilot instructions).
	if report.Steps[0].Status != domain.StepSuccess {
		t.Errorf("step 0 status = %q, want success", report.Steps[0].Status)
	}

	// Second action: failed (broken).
	if report.Steps[1].Status != domain.StepFailed {
		t.Errorf("step 1 status = %q, want failed", report.Steps[1].Status)
	}

	// Third action: rolled_back (never executed).
	if report.Steps[2].Status != domain.StepRolledBack {
		t.Errorf("step 2 status = %q, want rolled_back", report.Steps[2].Status)
	}
}

func TestExecute_WithBackup_AllSucceed(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	backupDir := t.TempDir()
	adapter := opencode.New()

	store := backup.NewStore(backupDir)

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
		cfg.Copilot,
		store,
	)

	report, execErr := exec.Execute(actions)
	if execErr != nil {
		t.Fatalf("execute: %v", execErr)
	}

	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("all steps should succeed")
	}

	if report.BackupID == "" {
		t.Error("backup ID should be set even when all succeed")
	}

	// Verify backup manifest is "complete" (no rollback).
	manifest, _ := store.Get(report.BackupID)
	if manifest.Status != "complete" {
		t.Errorf("manifest status = %q, want complete", manifest.Status)
	}
}

// ─── ActionDelete ─────────────────────────────────────────────────────────────

// TestExecute_ActionDelete_RemovesFile verifies that an ActionDelete action
// removes the target file from disk.
func TestExecute_ActionDelete_RemovesFile(t *testing.T) {
	project := t.TempDir()

	// Create a file that the delete action should remove.
	target := filepath.Join(project, "stale-file.md")
	if err := os.WriteFile(target, []byte("stale content"), 0644); err != nil {
		t.Fatal(err)
	}

	actions := []domain.PlannedAction{
		{
			ID:          "opencode-stale-cleanup",
			Agent:       domain.AgentOpenCode,
			Action:      domain.ActionDelete,
			TargetPath:  target,
			Description: "remove stale file for disabled adapter",
		},
	}

	exec := New(
		map[domain.ComponentID]domain.ComponentInstaller{},
		copilot.New(),
		project,
		domain.CopilotConfig{},
		nil,
	)

	report, err := exec.Execute(actions)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !report.Success {
		t.Fatalf("expected success, got failure: %v", report.Steps[0].Error)
	}

	if len(report.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(report.Steps))
	}

	if report.Steps[0].Status != domain.StepSuccess {
		t.Errorf("step status = %q, want success", report.Steps[0].Status)
	}

	// Verify the file was actually removed.
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("expected file %q to be removed, but it still exists", target)
	}
}

// TestExecute_ActionDelete_NonexistentFile_NoError verifies that an ActionDelete
// action on a file that doesn't exist succeeds (idempotent).
func TestExecute_ActionDelete_NonexistentFile_NoError(t *testing.T) {
	project := t.TempDir()

	// Target path does NOT exist on disk.
	target := filepath.Join(project, "nonexistent-file.md")

	actions := []domain.PlannedAction{
		{
			ID:          "opencode-stale-cleanup",
			Agent:       domain.AgentOpenCode,
			Action:      domain.ActionDelete,
			TargetPath:  target,
			Description: "remove stale file (idempotent)",
		},
	}

	exec := New(
		map[domain.ComponentID]domain.ComponentInstaller{},
		copilot.New(),
		project,
		domain.CopilotConfig{},
		nil,
	)

	report, err := exec.Execute(actions)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !report.Success {
		t.Errorf("expected success for delete of nonexistent file, got failure: %v", report.Steps[0].Error)
	}

	if len(report.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(report.Steps))
	}

	if report.Steps[0].Status != domain.StepSuccess {
		t.Errorf("step status = %q, want success", report.Steps[0].Status)
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
