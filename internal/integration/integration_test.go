package integration_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alexmosquera/agent-manager-pro/internal/adapters/claude"
	"github.com/alexmosquera/agent-manager-pro/internal/adapters/codex"
	"github.com/alexmosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/alexmosquera/agent-manager-pro/internal/backup"
	"github.com/alexmosquera/agent-manager-pro/internal/components/copilot"
	"github.com/alexmosquera/agent-manager-pro/internal/components/memory"
	"github.com/alexmosquera/agent-manager-pro/internal/config"
	"github.com/alexmosquera/agent-manager-pro/internal/domain"
	"github.com/alexmosquera/agent-manager-pro/internal/marker"
	"github.com/alexmosquera/agent-manager-pro/internal/pipeline"
	"github.com/alexmosquera/agent-manager-pro/internal/planner"
	"github.com/alexmosquera/agent-manager-pro/internal/verify"
)

// TestFullRoundTrip_PlanApplyVerify exercises the complete flow:
//   1. Load/merge config
//   2. Plan actions
//   3. Execute plan
//   4. Verify results
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
		merged.Copilot.InstructionsTemplate,
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
	promptPath := adapter.SystemPromptFile(home)
	if _, err := os.Stat(promptPath); err != nil {
		t.Errorf("memory prompt file missing: %s", promptPath)
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
	config.WriteJSON(config.UserConfigPath(home), userCfg)
	projCfg := domain.DefaultProjectConfig()
	config.WriteJSON(config.ProjectConfigPath(project), projCfg)

	user, _ := config.LoadUser(home)
	proj, _ := config.LoadProject(project)
	merged := config.Merge(user, proj, nil)

	adapter := opencode.New()
	p := planner.New()

	// First apply.
	actions1, _ := p.Plan(merged, []domain.Adapter{adapter}, home, project)
	exec := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot.InstructionsTemplate, nil)
	exec.Execute(actions1)

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
	config.WriteJSON(config.UserConfigPath(home), userCfg)

	projCfg := domain.DefaultProjectConfig()
	config.WriteJSON(config.ProjectConfigPath(project), projCfg)

	// Policy locks opencode enabled.
	policyCfg := domain.DefaultPolicyConfig()
	config.WriteJSON(config.PolicyConfigPath(project), policyCfg)

	user, _ := config.LoadUser(home)
	proj, _ := config.LoadProject(project)
	policy, _ := config.LoadPolicy(project)
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
	vReport, _ := v.Verify(merged, nil, home, project)

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

	// Pre-populate system prompt with user content.
	promptPath := adapter.SystemPromptFile(home)
	os.MkdirAll(filepath.Dir(promptPath), 0755)
	os.WriteFile(promptPath, []byte("# My Custom Agent Rules\n\nDo not touch this.\n"), 0644)

	// Pre-populate copilot instructions with user content.
	copilotPath := filepath.Join(project, copilot.CopilotInstructionsPath)
	os.MkdirAll(filepath.Dir(copilotPath), 0755)
	os.WriteFile(copilotPath, []byte("# Project-specific rules\n\nCustom instructions.\n"), 0644)

	userCfg := domain.DefaultUserConfig()
	config.WriteJSON(config.UserConfigPath(home), userCfg)
	projCfg := domain.DefaultProjectConfig()
	config.WriteJSON(config.ProjectConfigPath(project), projCfg)

	user, _ := config.LoadUser(home)
	proj, _ := config.LoadProject(project)
	merged := config.Merge(user, proj, nil)

	p := planner.New()
	actions, _ := p.Plan(merged, []domain.Adapter{adapter}, home, project)

	exec := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot.InstructionsTemplate, nil)
	exec.Execute(actions)

	// Verify user content is preserved in system prompt.
	promptData, _ := os.ReadFile(promptPath)
	promptStr := string(promptData)
	if !strContains(promptStr, "# My Custom Agent Rules") {
		t.Error("user content in system prompt was clobbered")
	}
	if !strContains(promptStr, "Do not touch this.") {
		t.Error("user content in system prompt was clobbered")
	}
	if !marker.HasSection(promptStr, "memory") {
		t.Error("memory section should be injected")
	}

	// Verify user content is preserved in copilot instructions.
	copilotData, _ := os.ReadFile(copilotPath)
	copilotStr := string(copilotData)
	if !strContains(copilotStr, "# Project-specific rules") {
		t.Error("user content in copilot instructions was clobbered")
	}
	if !strContains(copilotStr, "Custom instructions.") {
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

	// Create file with outdated memory content.
	promptPath := adapter.SystemPromptFile(home)
	os.MkdirAll(filepath.Dir(promptPath), 0755)
	outdated := marker.InjectSection("", "memory", "old protocol")
	os.WriteFile(promptPath, []byte(outdated), 0644)

	userCfg := domain.DefaultUserConfig()
	config.WriteJSON(config.UserConfigPath(home), userCfg)
	projCfg := domain.DefaultProjectConfig()
	config.WriteJSON(config.ProjectConfigPath(project), projCfg)

	user, _ := config.LoadUser(home)
	proj, _ := config.LoadProject(project)
	merged := config.Merge(user, proj, nil)

	p := planner.New()
	actions, _ := p.Plan(merged, []domain.Adapter{adapter}, home, project)

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

	exec := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot.InstructionsTemplate, nil)
	report, execErr := exec.Execute(actions)
	if execErr != nil {
		t.Fatalf("execute: %v", execErr)
	}
	if !report.Success {
		t.Fatal("apply should succeed")
	}

	// Verify passes after update.
	v := verify.New()
	vReport, _ := v.Verify(merged, []domain.Adapter{adapter}, home, project)
	if !vReport.AllPass {
		for _, r := range vReport.Results {
			if !r.Passed {
				t.Errorf("check %q failed: %s", r.Check, r.Message)
			}
		}
	}

	// Confirm the content is now current.
	data, _ := os.ReadFile(promptPath)
	extracted := marker.ExtractSection(string(data), "memory")
	if extracted != memory.ProtocolTemplate() {
		t.Error("memory content should match current protocol template")
	}
}

// TestBackup_RollbackOnFailure verifies that when a step fails during apply
// with backup enabled, all managed files are restored to their pre-apply state.
func TestBackup_RollbackOnFailure(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	backupDir := t.TempDir()
	adapter := opencode.New()

	// Pre-create the memory prompt file with known content.
	promptPath := adapter.SystemPromptFile(home)
	os.MkdirAll(filepath.Dir(promptPath), 0755)
	originalContent := "# My original rules\n\nDo not change.\n"
	os.WriteFile(promptPath, []byte(originalContent), 0644)

	// Set up config.
	userCfg := domain.DefaultUserConfig()
	config.WriteJSON(config.UserConfigPath(home), userCfg)
	projCfg := domain.DefaultProjectConfig()
	config.WriteJSON(config.ProjectConfigPath(project), projCfg)

	user, _ := config.LoadUser(home)
	proj, _ := config.LoadProject(project)
	merged := config.Merge(user, proj, nil)

	p := planner.New()
	actions, _ := p.Plan(merged, []domain.Adapter{adapter}, home, project)

	// Append a broken action that will cause failure after real actions run.
	broken := domain.PlannedAction{
		ID:          "broken-step",
		Component:   domain.ComponentID("nonexistent"),
		Action:      domain.ActionCreate,
		TargetPath:  filepath.Join(project, "ghost.txt"),
		Description: "this will fail",
	}
	allActions := append(actions, broken)

	store := backup.NewStore(backupDir)
	exec := pipeline.New(
		p.ComponentInstallers(),
		p.CopilotManager(),
		project,
		merged.Copilot.InstructionsTemplate,
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
	manifest, _ := store.Get(report.BackupID)
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
	config.WriteJSON(config.UserConfigPath(home), userCfg)
	projCfg := domain.DefaultProjectConfig()
	config.WriteJSON(config.ProjectConfigPath(project), projCfg)

	user, _ := config.LoadUser(home)
	proj, _ := config.LoadProject(project)
	merged := config.Merge(user, proj, nil)

	p := planner.New()
	actions, _ := p.Plan(merged, []domain.Adapter{adapter}, home, project)

	store := backup.NewStore(backupDir)
	exec := pipeline.New(
		p.ComponentInstallers(),
		p.CopilotManager(),
		project,
		merged.Copilot.InstructionsTemplate,
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
	manifest, _ := store.Get(report.BackupID)
	if manifest.Status != "complete" {
		t.Errorf("manifest status = %q, want complete", manifest.Status)
	}
	if len(manifest.AffectedFiles) == 0 {
		t.Error("expected affected files in manifest")
	}

	// Now restore from the backup — files should revert to pre-apply state.
	err := store.Restore(report.BackupID)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}

	// The files that were created should now be gone (they didn't exist before).
	promptPath := adapter.SystemPromptFile(home)
	if _, err := os.Stat(promptPath); err == nil {
		t.Error("prompt file should have been removed (didn't exist before apply)")
	}
}

func strContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ─── Milestone D: Personal Lane Gate Tests ──────────────────────────────────

// TestPersonalLane_TeamModeUnaffected verifies that enabling personal adapters
// (Claude Code, Codex) in user config does not change team-baseline behavior.
// This is the Milestone D acceptance gate.
func TestPersonalLane_TeamModeUnaffected(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	// Create user config with personal adapters enabled.
	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentClaudeCode)] = domain.AdapterConfig{Enabled: true}
	userCfg.Adapters[string(domain.AgentCodex)] = domain.AdapterConfig{Enabled: true}
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
	user, _ := config.LoadUser(home)
	proj, _ := config.LoadProject(project)
	policy, _ := config.LoadPolicy(project)
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
	codexAdapter := codex.New()
	allAdapters := []domain.Adapter{ocAdapter, claudeAdapter, codexAdapter}

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
	exec := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot.InstructionsTemplate, nil)
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
	promptPath := ocAdapter.SystemPromptFile(home)
	if _, err := os.Stat(promptPath); err != nil {
		t.Errorf("team memory prompt file missing: %s", promptPath)
	}

	copilotPath := filepath.Join(project, copilot.CopilotInstructionsPath)
	if _, err := os.Stat(copilotPath); err != nil {
		t.Errorf("copilot instructions missing: %s", copilotPath)
	}

	// Verify: personal adapter files also exist (Claude and Codex).
	claudePrompt := claudeAdapter.SystemPromptFile(home)
	if _, err := os.Stat(claudePrompt); err != nil {
		t.Errorf("claude memory prompt file missing: %s", claudePrompt)
	}

	codexPrompt := codexAdapter.SystemPromptFile(home)
	if _, err := os.Stat(codexPrompt); err != nil {
		t.Errorf("codex memory prompt file missing: %s", codexPrompt)
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
	config.WriteJSON(config.UserConfigPath(home), userCfg)

	// Project config: standard.
	projCfg := domain.DefaultProjectConfig()
	config.WriteJSON(config.ProjectConfigPath(project), projCfg)

	// Policy: standard (no locked fields for personal adapters).
	policyCfg := domain.DefaultPolicyConfig()
	config.WriteJSON(config.PolicyConfigPath(project), policyCfg)

	user, _ := config.LoadUser(home)
	proj, _ := config.LoadProject(project)
	policy, _ := config.LoadPolicy(project)
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
	userCfg.Adapters[string(domain.AgentCodex)] = domain.AdapterConfig{Enabled: true}
	config.WriteJSON(config.UserConfigPath(home), userCfg)

	projCfg := domain.DefaultProjectConfig()
	config.WriteJSON(config.ProjectConfigPath(project), projCfg)

	user, _ := config.LoadUser(home)
	proj, _ := config.LoadProject(project)
	merged := config.Merge(user, proj, nil)

	ocAdapter := opencode.New()
	claudeAdapter := claude.New()
	codexAdapter := codex.New()
	adapters := []domain.Adapter{ocAdapter, claudeAdapter, codexAdapter}

	p := planner.New()

	// First apply.
	actions1, _ := p.Plan(merged, adapters, home, project)
	exec := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot.InstructionsTemplate, nil)
	exec.Execute(actions1)

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
