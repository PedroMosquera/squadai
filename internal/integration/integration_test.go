package integration_test

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
	"github.com/PedroMosquera/squadai/internal/backup"
	"github.com/PedroMosquera/squadai/internal/cli"
	"github.com/PedroMosquera/squadai/internal/components/copilot"
	"github.com/PedroMosquera/squadai/internal/components/memory"
	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/marker"
	"github.com/PedroMosquera/squadai/internal/pipeline"
	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/verify"
)

// TestFullRoundTrip_PlanApplyVerify exercises the complete flow:
//  1. Load/merge config
//  2. Plan actions
//  3. Execute plan
//  4. Verify results
//
// Uses a temp directory as both home and project to avoid touching real files.
func TestFullRoundTrip_PlanApplyVerify(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	// Set up user config.
	userCfg := domain.DefaultUserConfig()
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	// Set up project config.
	projCfg := domain.DefaultProjectConfig()
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	// Load and merge.
	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	// Plan.
	adapter := opencode.New()
	p := planner.New()
	actions, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if len(actions) == 0 {
		t.Fatal("expected at least one action from fresh setup")
	}

	// All actions should be create (fresh environment).
	for _, a := range actions {
		if a.Action != domain.ActionCreate && a.Action != domain.ActionUpdate {
			t.Errorf("expected create/update for fresh setup, got %q for %q", a.Action, a.ID)
		}
	}

	// Apply (no backup for this basic test).
	exec := pipeline.New(
		p.ComponentInstallers(),
		p.CopilotManager(),
		project,
		merged.Copilot,
		nil,
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
		t.Fatal("apply should succeed")
	}

	// Verify.
	v := verify.New()
	vReport, err := v.Verify(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !vReport.AllPass {
		for _, r := range vReport.Results {
			if !r.Passed {
				t.Errorf("verify check %q failed: %s", r.Check, r.Message)
			}
		}
	}

	// Check files actually exist on disk.
	memoryPath := filepath.Join(project, "AGENTS.md") // OpenCode memory targets project-level
	if _, err := os.Stat(memoryPath); err != nil {
		t.Errorf("memory file missing: %s", memoryPath)
	}

	copilotPath := filepath.Join(project, copilot.CopilotInstructionsPath)
	if _, err := os.Stat(copilotPath); err != nil {
		t.Errorf("copilot instructions missing: %s", copilotPath)
	}
}

// TestIdempotent_SecondApplyProducesAllSkips verifies that running apply
// twice produces no changes on the second run.
func TestIdempotent_SecondApplyProducesAllSkips(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	userCfg := domain.DefaultUserConfig()
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}
	projCfg := domain.DefaultProjectConfig()
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	adapter := opencode.New()
	p := planner.New()

	// First apply.
	actions1, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("first plan: %v", err)
	}
	exec := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot, nil)
	if _, err := exec.Execute(actions1); err != nil {
		t.Fatal(err)
	}

	// Second plan — everything should be skip.
	actions2, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("second plan: %v", err)
	}

	for _, a := range actions2 {
		if a.Action != domain.ActionSkip {
			t.Errorf("second plan: action %q = %q, want skip", a.ID, a.Action)
		}
	}
}

// TestPolicyLockedFields_Enforced verifies that policy-locked fields
// override user config and the resulting plan uses the policy values.
func TestPolicyLockedFields_Enforced(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	// User tries to disable opencode.
	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters["opencode"] = domain.AdapterConfig{Enabled: false}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}

	projCfg := domain.DefaultProjectConfig()
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	// Policy locks opencode enabled.
	policyCfg := domain.DefaultPolicyConfig()
	if err := config.WriteJSON(config.PolicyConfigPath(project), policyCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	policy, err := config.LoadPolicy(project)
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	merged := config.Merge(user, proj, policy)

	// Policy should override user's attempt to disable opencode.
	if !merged.Adapters["opencode"].Enabled {
		t.Error("policy should force opencode enabled")
	}

	if len(merged.Violations) == 0 {
		t.Error("expected at least one violation recorded")
	}

	// Verify that the verifier includes policy override info.
	v := verify.New()
	vReport, err := v.Verify(merged, nil, home, project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	hasPolicyCheck := false
	for _, r := range vReport.Results {
		if r.Check == "policy-override" {
			hasPolicyCheck = true
		}
	}
	if !hasPolicyCheck {
		t.Error("expected policy-override check in verify report")
	}
}

// TestUserContent_Preserved verifies that user-authored content in
// managed files is not clobbered by apply.
func TestUserContent_Preserved(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	// Pre-populate project-level AGENTS.md with user content.
	promptPath := filepath.Join(project, "AGENTS.md")
	if err := os.WriteFile(promptPath, []byte("# My Custom Agent Rules\n\nDo not touch this.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-populate copilot instructions with user content.
	copilotPath := filepath.Join(project, copilot.CopilotInstructionsPath)
	if err := os.MkdirAll(filepath.Dir(copilotPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(copilotPath, []byte("# Project-specific rules\n\nCustom instructions.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	userCfg := domain.DefaultUserConfig()
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}
	projCfg := domain.DefaultProjectConfig()
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	p := planner.New()
	actions, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	exec := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot, nil)
	if _, err := exec.Execute(actions); err != nil {
		t.Fatal(err)
	}

	// Verify user content is preserved in project-level AGENTS.md.
	promptData, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	promptStr := string(promptData)
	if !strings.Contains(promptStr, "# My Custom Agent Rules") {
		t.Error("user content in AGENTS.md was clobbered")
	}
	if !strings.Contains(promptStr, "Do not touch this.") {
		t.Error("user content in AGENTS.md was clobbered")
	}
	if !marker.HasSection(promptStr, "memory") {
		t.Error("memory section should be injected into AGENTS.md")
	}

	// Verify user content is preserved in copilot instructions.
	copilotData, err := os.ReadFile(copilotPath)
	if err != nil {
		t.Fatalf("read copilot instructions: %v", err)
	}
	copilotStr := string(copilotData)
	if !strings.Contains(copilotStr, "# Project-specific rules") {
		t.Error("user content in copilot instructions was clobbered")
	}
	if !strings.Contains(copilotStr, "Custom instructions.") {
		t.Error("user content in copilot instructions was clobbered")
	}
	if !marker.HasSection(copilotStr, copilot.SectionID) {
		t.Error("copilot section should be injected")
	}
}

// TestApplyThenVerify_UpdatedContent verifies that after updating a
// memory template, apply updates the file and verify passes.
func TestApplyThenVerify_UpdatedContent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	// Create file with outdated memory content at project-level.
	promptPath := filepath.Join(project, "AGENTS.md")
	outdated := marker.InjectSection("", "memory", "old protocol")
	if err := os.WriteFile(promptPath, []byte(outdated), 0644); err != nil {
		t.Fatal(err)
	}

	userCfg := domain.DefaultUserConfig()
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}
	projCfg := domain.DefaultProjectConfig()
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	p := planner.New()
	actions, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	// Should plan an update for memory (outdated) and create for copilot (missing).
	hasUpdate := false
	for _, a := range actions {
		if a.Component == domain.ComponentMemory && a.Action == domain.ActionUpdate {
			hasUpdate = true
		}
	}
	if !hasUpdate {
		t.Error("expected update action for outdated memory")
	}

	exec := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot, nil)
	report, execErr := exec.Execute(actions)
	if execErr != nil {
		t.Fatalf("execute: %v", execErr)
	}
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	// Verify passes after update.
	v := verify.New()
	vReport, err := v.Verify(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !vReport.AllPass {
		for _, r := range vReport.Results {
			if !r.Passed {
				t.Errorf("check %q failed: %s", r.Check, r.Message)
			}
		}
	}

	// Confirm the content is now current.
	data, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	extracted := marker.ExtractSection(string(data), "memory")
	if extracted != memory.TemplateForAgentID(domain.AgentOpenCode) {
		t.Error("memory content should match current OpenCode protocol template")
	}
}

// TestBackup_RollbackOnFailure verifies that when a step fails during apply
// with backup enabled, all managed files are restored to their pre-apply state.
func TestBackup_RollbackOnFailure(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	backupDir := t.TempDir()
	adapter := opencode.New()

	// Pre-create the project-level AGENTS.md with known content.
	promptPath := filepath.Join(project, "AGENTS.md")
	originalContent := "# My original rules\n\nDo not change.\n"
	if err := os.WriteFile(promptPath, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Set up config.
	userCfg := domain.DefaultUserConfig()
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}
	projCfg := domain.DefaultProjectConfig()
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	p := planner.New()
	actions, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	// Append a broken action that will cause failure after real actions run.
	broken := domain.PlannedAction{
		ID:          "broken-step",
		Component:   domain.ComponentID("nonexistent"),
		Action:      domain.ActionCreate,
		TargetPath:  filepath.Join(project, "ghost.txt"),
		Description: "this will fail",
	}
	allActions := make([]domain.PlannedAction, 0, len(actions)+1)
	allActions = append(allActions, actions...)
	allActions = append(allActions, broken)

	store := backup.NewStore(backupDir)
	exec := pipeline.New(
		p.ComponentInstallers(),
		p.CopilotManager(),
		project,
		merged.Copilot,
		store,
	)

	report, execErr := exec.Execute(allActions)
	if execErr != nil {
		t.Fatalf("execute returned unexpected error: %v", execErr)
	}

	if report.Success {
		t.Fatal("report should indicate failure")
	}

	if report.BackupID == "" {
		t.Fatal("backup ID should be set")
	}

	// Verify the original file content was restored.
	data, err := os.ReadFile(promptPath)
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	if string(data) != originalContent {
		t.Errorf("prompt content not restored:\n  got:  %q\n  want: %q", string(data), originalContent)
	}

	// Verify the copilot file (which was created by a successful step)
	// was removed during rollback (it didn't exist before).
	copilotPath := filepath.Join(project, copilot.CopilotInstructionsPath)
	if _, err := os.Stat(copilotPath); err == nil {
		t.Error("copilot instructions should have been removed during rollback (didn't exist before)")
	}

	// Verify manifest status.
	manifest, err := store.Get(report.BackupID)
	if err != nil {
		t.Fatalf("get backup: %v", err)
	}
	if manifest.Status != "rolled_back" {
		t.Errorf("manifest status = %q, want rolled_back", manifest.Status)
	}
}

// TestBackup_SuccessfulApplyKeepsBackup verifies that a successful apply
// with backup creates a "complete" manifest that can be used for later restore.
func TestBackup_SuccessfulApplyKeepsBackup(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	backupDir := t.TempDir()
	adapter := opencode.New()

	userCfg := domain.DefaultUserConfig()
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}
	projCfg := domain.DefaultProjectConfig()
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	p := planner.New()
	actions, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	store := backup.NewStore(backupDir)
	exec := pipeline.New(
		p.ComponentInstallers(),
		p.CopilotManager(),
		project,
		merged.Copilot,
		store,
	)

	report, execErr := exec.Execute(actions)
	if execErr != nil {
		t.Fatalf("execute: %v", execErr)
	}
	if !report.Success {
		t.Fatal("apply should succeed")
	}
	if report.BackupID == "" {
		t.Fatal("backup ID should be set")
	}

	// Manifest should be "complete".
	manifest, err := store.Get(report.BackupID)
	if err != nil {
		t.Fatalf("get backup: %v", err)
	}
	if manifest.Status != "complete" {
		t.Errorf("manifest status = %q, want complete", manifest.Status)
	}
	if len(manifest.AffectedFiles) == 0 {
		t.Error("expected affected files in manifest")
	}

	// Now restore from the backup — files should revert to pre-apply state.
	if err = store.Restore(report.BackupID); err != nil {
		t.Fatalf("restore: %v", err)
	}

	// The files that were created should now be gone (they didn't exist before).
	memoryPath := filepath.Join(project, "AGENTS.md")
	if _, err := os.Stat(memoryPath); err == nil {
		t.Error("memory file should have been removed (didn't exist before apply)")
	}
}

// ─── Milestone D: Personal Lane Gate Tests ──────────────────────────────────

// TestPersonalLane_TeamModeUnaffected verifies that enabling personal adapters
// (Claude Code) in user config does not change team-baseline behavior.
// This is the Milestone D acceptance gate.
func TestPersonalLane_TeamModeUnaffected(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	// Create user config with personal adapters enabled.
	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentClaudeCode)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	// Create project config (standard team setup).
	projCfg := domain.DefaultProjectConfig()
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	// Create team policy that locks opencode and memory.
	policyCfg := domain.DefaultPolicyConfig()
	if err := config.WriteJSON(config.PolicyConfigPath(project), policyCfg); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	// Load and merge.
	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	policy, err := config.LoadPolicy(project)
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	merged := config.Merge(user, proj, policy)

	// Verify team baseline is enforced despite personal adapters being enabled.
	if !merged.Adapters["opencode"].Enabled {
		t.Error("opencode must remain enabled under policy")
	}
	if !merged.Components[string(domain.ComponentMemory)].Enabled {
		t.Error("memory component must remain enabled under policy")
	}
	if merged.Copilot.InstructionsTemplate != "standard" {
		t.Errorf("copilot template = %q, want standard", merged.Copilot.InstructionsTemplate)
	}

	// Plan with only the OpenCode adapter (team-only scenario).
	ocAdapter := opencode.New()
	teamAdapters := []domain.Adapter{ocAdapter}

	p := planner.New()
	teamActions, err := p.Plan(merged, teamAdapters, home, project)
	if err != nil {
		t.Fatalf("plan team-only: %v", err)
	}

	// Plan with OpenCode + personal adapters (hybrid scenario).
	claudeAdapter := claude.New()
	allAdapters := []domain.Adapter{ocAdapter, claudeAdapter}

	hybridActions, err := p.Plan(merged, allAdapters, home, project)
	if err != nil {
		t.Fatalf("plan hybrid: %v", err)
	}

	// The hybrid plan should include the same team actions plus personal ones.
	// Extract team-only actions from both plans for comparison.
	teamOnly := filterByAgent(teamActions, domain.AgentOpenCode)
	teamFromHybrid := filterByAgent(hybridActions, domain.AgentOpenCode)

	if len(teamOnly) != len(teamFromHybrid) {
		t.Errorf("team actions changed: team-only=%d, from-hybrid=%d", len(teamOnly), len(teamFromHybrid))
	}

	for i := range teamOnly {
		if i >= len(teamFromHybrid) {
			break
		}
		if teamOnly[i].ID != teamFromHybrid[i].ID {
			t.Errorf("team action[%d] ID mismatch: %q vs %q", i, teamOnly[i].ID, teamFromHybrid[i].ID)
		}
		if teamOnly[i].Action != teamFromHybrid[i].Action {
			t.Errorf("team action[%d] type mismatch: %q vs %q", i, teamOnly[i].Action, teamFromHybrid[i].Action)
		}
		if teamOnly[i].TargetPath != teamFromHybrid[i].TargetPath {
			t.Errorf("team action[%d] path mismatch: %q vs %q", i, teamOnly[i].TargetPath, teamFromHybrid[i].TargetPath)
		}
	}

	// Also check copilot action is present in both plans.
	teamCopilot := filterByCopilot(teamActions)
	hybridCopilot := filterByCopilot(hybridActions)
	if len(teamCopilot) != len(hybridCopilot) {
		t.Errorf("copilot actions changed: team=%d, hybrid=%d", len(teamCopilot), len(hybridCopilot))
	}

	// Execute the hybrid plan to verify it succeeds end-to-end.
	exec := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot, nil)
	report, execErr := exec.Execute(hybridActions)
	if execErr != nil {
		t.Fatalf("hybrid execute: %v", execErr)
	}
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("hybrid apply should succeed")
	}

	// Verify: team files exist.
	ocMemoryPath := filepath.Join(project, "AGENTS.md") // OpenCode targets project-level
	if _, err := os.Stat(ocMemoryPath); err != nil {
		t.Errorf("team memory file missing: %s", ocMemoryPath)
	}

	copilotPath := filepath.Join(project, copilot.CopilotInstructionsPath)
	if _, err := os.Stat(copilotPath); err != nil {
		t.Errorf("copilot instructions missing: %s", copilotPath)
	}

	// Verify: personal adapter files also exist.
	// Claude targets project-level CLAUDE.md.
	claudeMemoryPath := filepath.Join(project, "CLAUDE.md")
	if _, err := os.Stat(claudeMemoryPath); err != nil {
		t.Errorf("claude memory file missing: %s", claudeMemoryPath)
	}

	// Run verifier — all checks should pass.
	v := verify.New()
	vReport, err := v.Verify(merged, allAdapters, home, project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !vReport.AllPass {
		for _, r := range vReport.Results {
			if !r.Passed {
				t.Errorf("verify check %q failed: %s", r.Check, r.Message)
			}
		}
	}
}

// TestPersonalLane_PolicyCannotEnablePersonalAdapters verifies that policy
// locked fields prevent personal adapters from being force-enabled by policy.
// Personal adapters should only be controlled by user config.
func TestPersonalLane_PolicyCannotEnablePersonalAdapters(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	// User config: personal adapters disabled.
	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentClaudeCode)] = domain.AdapterConfig{Enabled: false}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}

	// Project config: standard.
	projCfg := domain.DefaultProjectConfig()
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	// Policy: standard (no locked fields for personal adapters).
	policyCfg := domain.DefaultPolicyConfig()
	if err := config.WriteJSON(config.PolicyConfigPath(project), policyCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	policy, err := config.LoadPolicy(project)
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	merged := config.Merge(user, proj, policy)

	// Personal adapter should remain disabled — user's choice.
	if merged.Adapters[string(domain.AgentClaudeCode)].Enabled {
		t.Error("claude-code should remain disabled (user choice)")
	}

	// Team adapter should be enabled by policy.
	if !merged.Adapters["opencode"].Enabled {
		t.Error("opencode must be enabled by policy")
	}
}

// TestPersonalLane_SecondApplyIdempotent verifies that running apply twice
// with personal adapters enabled still produces skip on second run.
func TestPersonalLane_SecondApplyIdempotent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentClaudeCode)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}

	projCfg := domain.DefaultProjectConfig()
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	ocAdapter := opencode.New()
	claudeAdapter := claude.New()
	adapters := []domain.Adapter{ocAdapter, claudeAdapter}

	p := planner.New()

	// First apply.
	actions1, err := p.Plan(merged, adapters, home, project)
	if err != nil {
		t.Fatalf("first plan: %v", err)
	}
	exec := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot, nil)
	if _, err := exec.Execute(actions1); err != nil {
		t.Fatal(err)
	}

	// Second plan — all should be skip.
	actions2, err := p.Plan(merged, adapters, home, project)
	if err != nil {
		t.Fatalf("second plan: %v", err)
	}

	for _, a := range actions2 {
		if a.Action != domain.ActionSkip {
			t.Errorf("second plan: action %q = %q, want skip", a.ID, a.Action)
		}
	}
}

func filterByAgent(actions []domain.PlannedAction, agent domain.AgentID) []domain.PlannedAction {
	var result []domain.PlannedAction
	for _, a := range actions {
		if a.Agent == agent {
			result = append(result, a)
		}
	}
	return result
}

func filterByCopilot(actions []domain.PlannedAction) []domain.PlannedAction {
	var result []domain.PlannedAction
	for _, a := range actions {
		if a.Component == domain.ComponentID("copilot") {
			result = append(result, a)
		}
	}
	return result
}

// ─── V2 Helper Functions ──────────────────────────────────────────────────────

// assertFileExists checks that a file exists at the given path.
func assertFileExists(t *testing.T, path, label string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("%s: expected file to exist at %s", label, path)
	}
}

// assertFileContains checks that a file exists and contains the given substring.
func assertFileContains(t *testing.T, path, needle, label string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: failed to read %s: %v", label, path, err)
	}
	if !strings.Contains(string(data), needle) {
		t.Errorf("%s: expected %s to contain %q", label, path, needle)
	}
}

// assertJSONKey checks that a JSON file contains the given key string.
func assertJSONKey(t *testing.T, path, key, label string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: failed to read %s: %v", label, path, err)
	}
	if !strings.Contains(string(data), `"`+key+`"`) {
		t.Errorf("%s: expected %s to contain JSON key %q", label, path, key)
	}
}

// buildTDDConfig writes and loads a TDD methodology config for OpenCode.
func buildTDDConfig(t *testing.T, home, project string) *domain.MergedConfig {
	t.Helper()
	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentOpenCode): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentAgents): {Enabled: true},
			string(domain.ComponentSkills): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyTDD,
		Team:        domain.DefaultTeam(domain.MethodologyTDD),
		MCP:         cli.DefaultMCPServers(),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	return config.Merge(user, proj, nil)
}

// buildSDDClaudeConfig writes and loads an SDD methodology config for Claude Code.
func buildSDDClaudeConfig(t *testing.T, home, project string) *domain.MergedConfig {
	t.Helper()
	userCfg := domain.DefaultUserConfig()
	// Disable opencode so verify doesn't fail on undetected opencode binary.
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: false}
	userCfg.Adapters[string(domain.AgentClaudeCode)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentClaudeCode): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologySDD,
		Team:        domain.DefaultTeam(domain.MethodologySDD),
		MCP:         cli.DefaultMCPServers(),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	return config.Merge(user, proj, nil)
}

// buildConventionalConfig writes and loads a Conventional methodology config for OpenCode.
func buildConventionalConfig(t *testing.T, home, project string) *domain.MergedConfig {
	t.Helper()
	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentOpenCode): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentAgents): {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyConventional,
		Team:        domain.DefaultTeam(domain.MethodologyConventional),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	return config.Merge(user, proj, nil)
}

// runPlanExecute runs plan + execute for an adapter with the given merged config.
func runPlanExecute(t *testing.T, merged *domain.MergedConfig, adapter domain.Adapter, home, project string) *domain.ApplyReport {
	t.Helper()
	p := planner.New()
	actions, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	exec := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot, nil)
	report, err := exec.Execute(actions)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	return report
}

// ─── Group A: Full Pipeline per Methodology ───────────────────────────────────

// TestFullPipeline_TDD_OpenCode verifies TDD + OpenCode: agents dir, memory marker, MCP.
func TestFullPipeline_TDD_OpenCode(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	merged := buildTDDConfig(t, home, project)
	adapter := opencode.New()

	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("apply should succeed")
	}

	// AGENTS.md must exist with memory marker.
	memPath := filepath.Join(project, "AGENTS.md")
	assertFileExists(t, memPath, "TDD/OpenCode: AGENTS.md")
	assertFileContains(t, memPath, "squadai", "TDD/OpenCode: AGENTS.md has marker")

	// .opencode/agents/ dir with orchestrator and TDD roles.
	agentsDir := filepath.Join(project, ".opencode", "agents")
	assertFileExists(t, filepath.Join(agentsDir, "orchestrator.md"), "TDD/OpenCode: orchestrator.md")
	assertFileExists(t, filepath.Join(agentsDir, "brainstormer.md"), "TDD/OpenCode: brainstormer.md")
	assertFileExists(t, filepath.Join(agentsDir, "planner.md"), "TDD/OpenCode: planner.md")
	assertFileExists(t, filepath.Join(agentsDir, "implementer.md"), "TDD/OpenCode: implementer.md")
	assertFileExists(t, filepath.Join(agentsDir, "reviewer.md"), "TDD/OpenCode: reviewer.md")
	assertFileExists(t, filepath.Join(agentsDir, "debugger.md"), "TDD/OpenCode: debugger.md")

	// opencode.json must have "mcp" key with context7.
	ocJSON := filepath.Join(project, "opencode.json")
	assertFileExists(t, ocJSON, "TDD/OpenCode: opencode.json")
	assertJSONKey(t, ocJSON, "mcp", "TDD/OpenCode: opencode.json has mcp key")
	assertFileContains(t, ocJSON, "context7", "TDD/OpenCode: opencode.json has context7")

	// Verify passes.
	v := verify.New()
	vReport, err := v.Verify(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !vReport.AllPass {
		for _, r := range vReport.Results {
			if !r.Passed {
				t.Errorf("verify check %q failed: %s", r.Check, r.Message)
			}
		}
	}
}

// TestFullPipeline_SDD_ClaudeCode verifies SDD + Claude Code: CLAUDE.md, separate MCP files.
func TestFullPipeline_SDD_ClaudeCode(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	merged := buildSDDClaudeConfig(t, home, project)
	adapter := claude.New()

	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("apply should succeed")
	}

	// CLAUDE.md must exist with memory marker.
	claudeMD := filepath.Join(project, "CLAUDE.md")
	assertFileExists(t, claudeMD, "SDD/Claude: CLAUDE.md")
	assertFileContains(t, claudeMD, "squadai", "SDD/Claude: CLAUDE.md has marker")

	// ~/.claude/mcp/context7.json must exist (SeparateMCPFiles strategy).
	mcpFile := filepath.Join(home, ".claude", "mcp", "context7.json")
	assertFileExists(t, mcpFile, "SDD/Claude: context7.json MCP file")
	assertJSONKey(t, mcpFile, "command", "SDD/Claude: context7.json has command")

	// Verify passes.
	v := verify.New()
	vReport, err := v.Verify(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !vReport.AllPass {
		for _, r := range vReport.Results {
			if !r.Passed {
				t.Errorf("verify check %q failed: %s", r.Check, r.Message)
			}
		}
	}
}

// TestFullPipeline_Conventional_OpenCode verifies Conventional + OpenCode: 4 role files.
func TestFullPipeline_Conventional_OpenCode(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	merged := buildConventionalConfig(t, home, project)
	adapter := opencode.New()

	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("apply should succeed")
	}

	// Verify 4 conventional role files under .opencode/agents/.
	agentsDir := filepath.Join(project, ".opencode", "agents")
	assertFileExists(t, filepath.Join(agentsDir, "orchestrator.md"), "Conv/OpenCode: orchestrator.md")
	assertFileExists(t, filepath.Join(agentsDir, "implementer.md"), "Conv/OpenCode: implementer.md")
	assertFileExists(t, filepath.Join(agentsDir, "reviewer.md"), "Conv/OpenCode: reviewer.md")
	assertFileExists(t, filepath.Join(agentsDir, "tester.md"), "Conv/OpenCode: tester.md")

	// Verify passes.
	v := verify.New()
	vReport, err := v.Verify(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !vReport.AllPass {
		for _, r := range vReport.Results {
			if !r.Passed {
				t.Errorf("verify check %q failed: %s", r.Check, r.Message)
			}
		}
	}
}

// TestFullPipeline_TDD_Idempotent verifies second plan produces all ActionSkip.
func TestFullPipeline_TDD_Idempotent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	merged := buildTDDConfig(t, home, project)
	adapter := opencode.New()

	// First apply.
	p := planner.New()
	actions1, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("first plan: %v", err)
	}
	exec := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot, nil)
	if _, err := exec.Execute(actions1); err != nil {
		t.Fatalf("first execute: %v", err)
	}

	// Second plan — all should be skip.
	actions2, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("second plan: %v", err)
	}
	for _, a := range actions2 {
		if a.Action != domain.ActionSkip {
			t.Errorf("TDD idempotent: action %q = %q, want skip", a.ID, a.Action)
		}
	}
}

// TestFullPipeline_SDD_Idempotent verifies second plan produces all ActionSkip (SDD + Claude).
func TestFullPipeline_SDD_Idempotent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	merged := buildSDDClaudeConfig(t, home, project)
	adapter := claude.New()

	// First apply.
	p := planner.New()
	actions1, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("first plan: %v", err)
	}
	exec := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot, nil)
	if _, err := exec.Execute(actions1); err != nil {
		t.Fatalf("first execute: %v", err)
	}

	// Second plan — all should be skip.
	actions2, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("second plan: %v", err)
	}
	for _, a := range actions2 {
		if a.Action != domain.ActionSkip {
			t.Errorf("SDD idempotent: action %q = %q, want skip", a.ID, a.Action)
		}
	}
}

// ─── Group B: MCP Strategy Tests ──────────────────────────────────────────────

// TestMCPInstallation_OpenCode_MergeIntoSettings verifies OpenCode uses "mcp" key.
func TestMCPInstallation_OpenCode_MergeIntoSettings(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	merged := buildTDDConfig(t, home, project)
	adapter := opencode.New()

	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	ocJSON := filepath.Join(project, "opencode.json")
	assertFileExists(t, ocJSON, "MCP/OpenCode: opencode.json")
	assertJSONKey(t, ocJSON, "mcp", "MCP/OpenCode: has 'mcp' key (not mcpServers)")
	assertFileContains(t, ocJSON, "context7", "MCP/OpenCode: context7 present")
	assertFileContains(t, ocJSON, "npx", "MCP/OpenCode: npx command present")

	// Must NOT have mcpServers key.
	data, err := os.ReadFile(ocJSON)
	if err != nil {
		t.Fatalf("read opencode.json: %v", err)
	}
	if strings.Contains(string(data), `"mcpServers"`) {
		t.Error("MCP/OpenCode: opencode.json must not have 'mcpServers' key")
	}
}

// TestMCPInstallation_ClaudeCode_SeparateFiles verifies Claude creates ~/.claude/mcp/{name}.json.
func TestMCPInstallation_ClaudeCode_SeparateFiles(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	merged := buildSDDClaudeConfig(t, home, project)
	adapter := claude.New()

	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	// ~/.claude/mcp/context7.json must exist.
	mcpFile := filepath.Join(home, ".claude", "mcp", "context7.json")
	assertFileExists(t, mcpFile, "MCP/Claude: ~/.claude/mcp/context7.json")
	assertJSONKey(t, mcpFile, "command", "MCP/Claude: context7.json has command array")
}

// TestMCPInstallation_VSCode_MCPConfigFile verifies VS Code uses .vscode/mcp.json with mcpServers.
func TestMCPInstallation_VSCode_MCPConfigFile(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentVSCodeCopilot)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentVSCodeCopilot): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
		},
		Copilot: domain.CopilotConfig{InstructionsTemplate: "standard"},
		MCP:     cli.DefaultMCPServers(),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	adapter := vscode.New()
	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	// .vscode/mcp.json must exist (NOT settings.json).
	mcpJSON := filepath.Join(project, ".vscode", "mcp.json")
	assertFileExists(t, mcpJSON, "MCP/VSCode: .vscode/mcp.json")
	assertJSONKey(t, mcpJSON, "mcpServers", "MCP/VSCode: has 'mcpServers' key")

	// Must NOT use the "mcp" key.
	data, err := os.ReadFile(mcpJSON)
	if err != nil {
		t.Fatalf("read mcp.json: %v", err)
	}
	if strings.Contains(string(data), `"mcp":`) {
		t.Error("MCP/VSCode: .vscode/mcp.json must not have 'mcp' key (should be 'mcpServers')")
	}
}

// TestMCPInstallation_Cursor_MCPConfigFile verifies Cursor uses .cursor/mcp.json with mcpServers.
func TestMCPInstallation_Cursor_MCPConfigFile(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentCursor)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentCursor): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
		},
		Copilot: domain.CopilotConfig{InstructionsTemplate: "standard"},
		MCP:     cli.DefaultMCPServers(),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	adapter := cursor.New()
	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	// .cursor/mcp.json must exist with mcpServers key.
	mcpJSON := filepath.Join(project, ".cursor", "mcp.json")
	assertFileExists(t, mcpJSON, "MCP/Cursor: .cursor/mcp.json")
	assertJSONKey(t, mcpJSON, "mcpServers", "MCP/Cursor: has 'mcpServers' key")
}

// TestMCPInstallation_Windsurf_MCPConfigFile verifies Windsurf uses .windsurf/mcp_config.json.
func TestMCPInstallation_Windsurf_MCPConfigFile(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentWindsurf)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentWindsurf): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
		},
		Copilot: domain.CopilotConfig{InstructionsTemplate: "standard"},
		MCP:     cli.DefaultMCPServers(),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	adapter := windsurf.New()
	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	// .windsurf/mcp_config.json must exist with mcpServers key.
	mcpJSON := filepath.Join(project, ".windsurf", "mcp_config.json")
	assertFileExists(t, mcpJSON, "MCP/Windsurf: .windsurf/mcp_config.json")
	assertJSONKey(t, mcpJSON, "mcpServers", "MCP/Windsurf: has 'mcpServers' key")
}

// ─── Group C: Plugin & Workflow Tests ─────────────────────────────────────────

// TestPluginInstallation_ClaudeCode_EnabledPlugins verifies claude_plugin writes to enabledPlugins.
func TestPluginInstallation_ClaudeCode_EnabledPlugins(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentClaudeCode)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentClaudeCode): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory):  {Enabled: true},
			string(domain.ComponentPlugins): {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologySDD, // SDD allows superpowers
		Plugins: map[string]domain.PluginDef{
			"superpowers": {
				Description:     "Test plugin",
				Enabled:         true,
				SupportedAgents: []string{"claude-code"},
				InstallMethod:   "claude_plugin",
				PluginID:        "superpowers@claude-plugins-official",
			},
		},
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	adapter := claude.New()
	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	// ~/.claude/settings.json must have enabledPlugins key.
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	assertFileExists(t, settingsPath, "Plugin/Claude: settings.json")
	assertJSONKey(t, settingsPath, "enabledPlugins", "Plugin/Claude: enabledPlugins key present")
}

// TestPluginInstallation_Superpowers_TDD_Excluded verifies superpowers is blocked for TDD.
func TestPluginInstallation_Superpowers_TDD_Excluded(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentClaudeCode)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentClaudeCode): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentPlugins): {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyTDD, // TDD blocks superpowers
		Plugins: map[string]domain.PluginDef{
			"superpowers": {
				Description:         "Test plugin",
				Enabled:             true,
				SupportedAgents:     []string{"claude-code"},
				InstallMethod:       "claude_plugin",
				PluginID:            "superpowers@claude-plugins-official",
				ExcludesMethodology: "tdd",
			},
		},
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	adapter := claude.New()
	p := planner.New()
	actions, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	// Must have zero plugin actions (superpowers excluded by TDD).
	pluginActions := filterByComponent(actions, domain.ComponentPlugins)
	if len(pluginActions) != 0 {
		t.Errorf("expected 0 plugin actions for TDD+superpowers, got %d", len(pluginActions))
	}
}

// TestWorkflowInstallation_Windsurf_SDD verifies SDD workflow file created for Windsurf.
func TestWorkflowInstallation_Windsurf_SDD(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentWindsurf)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentWindsurf): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory):    {Enabled: true},
			string(domain.ComponentWorkflows): {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologySDD,
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	adapter := windsurf.New()
	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	// .windsurf/workflows/sdd-pipeline.md must exist and be non-empty.
	workflowPath := filepath.Join(project, ".windsurf", "workflows", "sdd-pipeline.md")
	assertFileExists(t, workflowPath, "Workflow/Windsurf/SDD: sdd-pipeline.md")
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	if len(data) == 0 {
		t.Error("Workflow/Windsurf/SDD: sdd-pipeline.md must be non-empty")
	}
}

// TestWorkflowInstallation_NonWindsurf_NoActions verifies non-Windsurf adapters get no workflow actions.
func TestWorkflowInstallation_NonWindsurf_NoActions(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentOpenCode): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory):    {Enabled: true},
			string(domain.ComponentWorkflows): {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyTDD,
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	adapter := opencode.New()
	p := planner.New()
	actions, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	// Must have zero workflow actions (OpenCode doesn't support workflows).
	workflowActions := filterByComponent(actions, domain.ComponentWorkflows)
	if len(workflowActions) != 0 {
		t.Errorf("expected 0 workflow actions for OpenCode, got %d", len(workflowActions))
	}
}

// ─── Group D: Team Composition Verification ──────────────────────────────────

// TestTeamComposition_TDD_AllRolesPresent verifies all 6 TDD role files created.
func TestTeamComposition_TDD_AllRolesPresent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	merged := buildTDDConfig(t, home, project)
	adapter := opencode.New()

	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	agentsDir := filepath.Join(project, ".opencode", "agents")
	tddRoles := []string{"orchestrator.md", "brainstormer.md", "planner.md", "implementer.md", "reviewer.md", "debugger.md"}
	for _, role := range tddRoles {
		assertFileExists(t, filepath.Join(agentsDir, role), "TDD team: "+role)
	}
}

// TestTeamComposition_SDD_AllRolesPresent verifies all 8 SDD role files created.
func TestTeamComposition_SDD_AllRolesPresent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentOpenCode): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentAgents): {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologySDD,
		Team:        domain.DefaultTeam(domain.MethodologySDD),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	adapter := opencode.New()
	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	agentsDir := filepath.Join(project, ".opencode", "agents")
	sddRoles := []string{"orchestrator.md", "explorer.md", "proposer.md", "spec-writer.md", "designer.md", "task-planner.md", "implementer.md", "verifier.md"}
	for _, role := range sddRoles {
		assertFileExists(t, filepath.Join(agentsDir, role), "SDD team: "+role)
	}
}

// TestTeamComposition_Conventional_AllRolesPresent verifies all 4 Conventional role files.
func TestTeamComposition_Conventional_AllRolesPresent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	merged := buildConventionalConfig(t, home, project)
	adapter := opencode.New()

	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	agentsDir := filepath.Join(project, ".opencode", "agents")
	convRoles := []string{"orchestrator.md", "implementer.md", "reviewer.md", "tester.md"}
	for _, role := range convRoles {
		assertFileExists(t, filepath.Join(agentsDir, role), "Conventional team: "+role)
	}
}

// ─── Group E: Cross-cutting Tests ─────────────────────────────────────────────

// TestVerify_ReportsAllComponents verifies the report covers multiple component types.
func TestVerify_ReportsAllComponents(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	merged := buildTDDConfig(t, home, project)
	adapter := opencode.New()

	// Apply first.
	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	// Verify.
	v := verify.New()
	vReport, err := v.Verify(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}

	// Collect unique component types from results.
	components := make(map[string]bool)
	for _, r := range vReport.Results {
		if r.Component != "" {
			components[r.Component] = true
		}
	}

	// With memory + agents + mcp enabled, we expect at least memory and mcp components.
	if !components["memory"] {
		t.Error("verify report should include memory component checks")
	}
	if !components["mcp"] {
		t.Error("verify report should include mcp component checks")
	}
	if len(components) < 2 {
		t.Errorf("expected at least 2 component types in verify report, got %d: %v", len(components), components)
	}
}

// TestMultiAdapter_PlanCoversAll verifies plan covers both OpenCode and Claude adapters.
func TestMultiAdapter_PlanCoversAll(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: true}
	userCfg.Adapters[string(domain.AgentClaudeCode)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentOpenCode):   {Enabled: true},
			string(domain.AgentClaudeCode): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyConventional,
		MCP:         cli.DefaultMCPServers(),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	ocAdapter := opencode.New()
	ccAdapter := claude.New()
	adapters := []domain.Adapter{ocAdapter, ccAdapter}

	p := planner.New()
	actions, err := p.Plan(merged, adapters, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	// Must have actions for both agents.
	ocActions := filterByAgent(actions, domain.AgentOpenCode)
	ccActions := filterByAgent(actions, domain.AgentClaudeCode)

	if len(ocActions) == 0 {
		t.Error("expected actions for OpenCode adapter")
	}
	if len(ccActions) == 0 {
		t.Error("expected actions for Claude Code adapter")
	}

	// Verify OpenCode targets project-level paths (e.g., opencode.json, AGENTS.md).
	hasOpenCodePath := false
	for _, a := range ocActions {
		if strings.Contains(a.TargetPath, project) {
			hasOpenCodePath = true
			break
		}
	}
	if !hasOpenCodePath {
		t.Error("expected OpenCode actions to target project paths")
	}

	// Verify Claude Code targets home-level MCP paths (~/.claude/mcp/).
	hasClaudeMCPPath := false
	for _, a := range ccActions {
		if strings.Contains(a.TargetPath, ".claude") {
			hasClaudeMCPPath = true
			break
		}
	}
	if !hasClaudeMCPPath {
		t.Error("expected Claude Code actions to target ~/.claude paths")
	}
}

// ─── Additional Filter Helper ─────────────────────────────────────────────────

// TestFullPipeline_Cursor_Conventional verifies Conventional + Cursor: agents dir targets .cursor/agents/.
func TestFullPipeline_Cursor_Conventional(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentCursor)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentCursor): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentAgents): {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyConventional,
		Team:        domain.DefaultTeam(domain.MethodologyConventional),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	adapter := cursor.New()

	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		for _, s := range report.Steps {
			if s.Status == domain.StepFailed {
				t.Errorf("step %q failed: %s", s.Action.ID, s.Error)
			}
		}
		t.Fatal("apply should succeed")
	}

	// Agent installation actions must target .cursor/agents/ directory.
	p := planner.New()
	actions, err := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("second plan (post-apply): %v", err)
	}
	for _, a := range actions {
		if a.Component == domain.ComponentAgents && a.Action != domain.ActionSkip {
			t.Errorf("expected skip on second plan for agents, got %q for %q", a.Action, a.ID)
		}
		if a.Component == domain.ComponentAgents && !strings.Contains(a.TargetPath, ".cursor/agents") {
			t.Errorf("agent action %q targets %q, want path under .cursor/agents", a.ID, a.TargetPath)
		}
	}

	// Verify DelegationNativeAgents strategy writes role files under .cursor/agents/.
	agentsDir := filepath.Join(project, ".cursor", "agents")
	convRoles := []string{"orchestrator.md", "implementer.md", "reviewer.md", "tester.md"}
	for _, role := range convRoles {
		assertFileExists(t, filepath.Join(agentsDir, role), "Cursor/Conv: "+role)
	}
}

// TestWorkflowInstallation_Windsurf_TDD verifies TDD workflow file created for Windsurf.
func TestWorkflowInstallation_Windsurf_TDD(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentWindsurf)] = domain.AdapterConfig{Enabled: true}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatal(err)
	}

	projCfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentWindsurf): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory):    {Enabled: true},
			string(domain.ComponentWorkflows): {Enabled: true},
		},
		Copilot:     domain.CopilotConfig{InstructionsTemplate: "standard"},
		Methodology: domain.MethodologyTDD,
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatal(err)
	}

	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	merged := config.Merge(user, proj, nil)

	adapter := windsurf.New()
	report := runPlanExecute(t, merged, adapter, home, project)
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	// .windsurf/workflows/tdd-pipeline.md must exist and be non-empty.
	workflowPath := filepath.Join(project, ".windsurf", "workflows", "tdd-pipeline.md")
	assertFileExists(t, workflowPath, "Workflow/Windsurf/TDD: tdd-pipeline.md")
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	if len(data) == 0 {
		t.Error("Workflow/Windsurf/TDD: tdd-pipeline.md must be non-empty")
	}
	// TDD workflow should mention test-related content.
	assertFileContains(t, workflowPath, "TDD", "Workflow/Windsurf/TDD: tdd-pipeline.md contains TDD content")
}

func filterByComponent(actions []domain.PlannedAction, component domain.ComponentID) []domain.PlannedAction {
	var result []domain.PlannedAction
	for _, a := range actions {
		if a.Component == component {
			result = append(result, a)
		}
	}
	return result
}
