package config

import (
	"strings"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// ─── Merge precedence tests ─────────────────────────────────────────────────

func TestMerge_AllNil_ReturnsDefaults(t *testing.T) {
	merged := Merge(nil, nil, nil)

	if merged.Mode != domain.ModePersonal {
		t.Errorf("Mode = %q, want %q", merged.Mode, domain.ModePersonal)
	}
	if len(merged.Adapters) != 0 {
		t.Errorf("Adapters count = %d, want 0", len(merged.Adapters))
	}
	if len(merged.Components) != 0 {
		t.Errorf("Components count = %d, want 0", len(merged.Components))
	}
	if merged.Paths.BackupDir != "~/.agent-manager/backups" {
		t.Errorf("BackupDir = %q, want default", merged.Paths.BackupDir)
	}
	if len(merged.Violations) != 0 {
		t.Errorf("Violations = %v, want empty", merged.Violations)
	}
}

func TestMerge_UserOnly_SetsUserValues(t *testing.T) {
	user := &domain.UserConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"opencode":    {Enabled: true},
			"claude-code": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Paths: domain.PathsConfig{
			BackupDir: "/custom/backups",
		},
	}

	merged := Merge(user, nil, nil)

	if merged.Mode != domain.ModePersonal {
		t.Errorf("Mode = %q, want %q", merged.Mode, domain.ModePersonal)
	}
	if !merged.Adapters["opencode"].Enabled {
		t.Error("opencode adapter should be enabled")
	}
	if !merged.Adapters["claude-code"].Enabled {
		t.Error("claude-code adapter should be enabled")
	}
	if !merged.Components["memory"].Enabled {
		t.Error("memory component should be enabled")
	}
	if merged.Paths.BackupDir != "/custom/backups" {
		t.Errorf("BackupDir = %q, want /custom/backups", merged.Paths.BackupDir)
	}
}

func TestMerge_ProjectOverridesUser(t *testing.T) {
	user := &domain.UserConfig{
		Mode: domain.ModePersonal,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: false},
		},
	}
	project := &domain.ProjectConfig{
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Copilot: domain.CopilotConfig{
			InstructionsTemplate: "project-template",
		},
	}

	merged := Merge(user, project, nil)

	if !merged.Components["memory"].Enabled {
		t.Error("project should override user: memory should be enabled")
	}
	if merged.Copilot.InstructionsTemplate != "project-template" {
		t.Errorf("InstructionsTemplate = %q, want %q", merged.Copilot.InstructionsTemplate, "project-template")
	}
}

func TestMerge_PolicyOverridesAll(t *testing.T) {
	user := &domain.UserConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: false},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: false},
		},
	}
	project := &domain.ProjectConfig{
		Copilot: domain.CopilotConfig{
			InstructionsTemplate: "project-template",
		},
	}
	policy := &domain.PolicyConfig{
		Mode: domain.ModeTeam,
		Locked: []string{
			"adapters.opencode.enabled",
			"components.memory.enabled",
			"copilot.instructions_template",
		},
		Required: domain.RequiredBlock{
			Adapters: map[string]domain.AdapterConfig{
				"opencode": {Enabled: true},
			},
			Components: map[string]domain.ComponentConfig{
				"memory": {Enabled: true},
			},
			Copilot: domain.CopilotConfig{
				InstructionsTemplate: "policy-template",
			},
		},
	}

	merged := Merge(user, project, policy)

	if merged.Mode != domain.ModeTeam {
		t.Errorf("Mode = %q, want %q (policy should override)", merged.Mode, domain.ModeTeam)
	}
	if !merged.Adapters["opencode"].Enabled {
		t.Error("policy should enforce opencode enabled")
	}
	if !merged.Components["memory"].Enabled {
		t.Error("policy should enforce memory enabled")
	}
	if merged.Copilot.InstructionsTemplate != "policy-template" {
		t.Errorf("InstructionsTemplate = %q, want %q", merged.Copilot.InstructionsTemplate, "policy-template")
	}
}

// ─── Violation recording tests ──────────────────────────────────────────────

func TestMerge_RecordsViolation_WhenUserConflictsWithLockedAdapter(t *testing.T) {
	user := &domain.UserConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: false}, // conflicts with locked true
		},
	}
	policy := &domain.PolicyConfig{
		Mode: domain.ModeTeam,
		Locked: []string{
			"adapters.opencode.enabled",
		},
		Required: domain.RequiredBlock{
			Adapters: map[string]domain.AdapterConfig{
				"opencode": {Enabled: true},
			},
		},
	}

	merged := Merge(user, nil, policy)

	if len(merged.Violations) == 0 {
		t.Fatal("expected at least one violation, got none")
	}
	found := false
	for _, v := range merged.Violations {
		if strings.Contains(v, "adapters.opencode.enabled") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected violation about adapters.opencode.enabled, got: %v", merged.Violations)
	}
}

func TestMerge_RecordsViolation_WhenProjectConflictsWithLockedCopilot(t *testing.T) {
	user := &domain.UserConfig{
		Mode: domain.ModePersonal,
	}
	project := &domain.ProjectConfig{
		Copilot: domain.CopilotConfig{
			InstructionsTemplate: "project-override",
		},
	}
	policy := &domain.PolicyConfig{
		Mode: domain.ModeTeam,
		Locked: []string{
			"copilot.instructions_template",
		},
		Required: domain.RequiredBlock{
			Copilot: domain.CopilotConfig{
				InstructionsTemplate: "locked-template",
			},
		},
	}

	merged := Merge(user, project, policy)

	if merged.Copilot.InstructionsTemplate != "locked-template" {
		t.Errorf("InstructionsTemplate = %q, want %q", merged.Copilot.InstructionsTemplate, "locked-template")
	}
	if len(merged.Violations) == 0 {
		t.Fatal("expected violation for copilot.instructions_template, got none")
	}
}

func TestMerge_NoViolation_WhenValuesMatch(t *testing.T) {
	user := &domain.UserConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true}, // matches policy
		},
	}
	policy := &domain.PolicyConfig{
		Mode: domain.ModeTeam,
		Locked: []string{
			"adapters.opencode.enabled",
		},
		Required: domain.RequiredBlock{
			Adapters: map[string]domain.AdapterConfig{
				"opencode": {Enabled: true},
			},
		},
	}

	merged := Merge(user, nil, policy)

	if len(merged.Violations) != 0 {
		t.Errorf("expected no violations when values match, got: %v", merged.Violations)
	}
}

func TestMerge_NoViolation_WhenFieldNotLocked(t *testing.T) {
	user := &domain.UserConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: false},
		},
	}
	policy := &domain.PolicyConfig{
		Mode: domain.ModeTeam,
		Locked: []string{}, // nothing locked
		Required: domain.RequiredBlock{
			Adapters: map[string]domain.AdapterConfig{
				"opencode": {Enabled: true},
			},
		},
	}

	merged := Merge(user, nil, policy)

	// Policy required still wins, but no violation since field is not locked
	if !merged.Adapters["opencode"].Enabled {
		t.Error("policy required should still apply")
	}
	if len(merged.Violations) != 0 {
		t.Errorf("expected no violations when field is not locked, got: %v", merged.Violations)
	}
}

// ─── Policy-only merge tests ────────────────────────────────────────────────

func TestMerge_PolicyOnly_NoUserOrProject(t *testing.T) {
	policy := domain.DefaultPolicyConfig()

	merged := Merge(nil, nil, policy)

	if merged.Mode != domain.ModeTeam {
		t.Errorf("Mode = %q, want %q", merged.Mode, domain.ModeTeam)
	}
	if !merged.Adapters["opencode"].Enabled {
		t.Error("opencode should be enabled from policy required block")
	}
	if !merged.Components["memory"].Enabled {
		t.Error("memory should be enabled from policy required block")
	}
}

// ─── buildLockedSet tests ───────────────────────────────────────────────────

func TestBuildLockedSet_TrimWhitespace(t *testing.T) {
	set := buildLockedSet([]string{"  foo.bar  ", "baz.qux"})

	if _, ok := set["foo.bar"]; !ok {
		t.Error("expected trimmed key 'foo.bar' in set")
	}
	if _, ok := set["baz.qux"]; !ok {
		t.Error("expected 'baz.qux' in set")
	}
	if len(set) != 2 {
		t.Errorf("set size = %d, want 2", len(set))
	}
}

func TestBuildLockedSet_Empty(t *testing.T) {
	set := buildLockedSet(nil)
	if len(set) != 0 {
		t.Errorf("set size = %d, want 0 for nil input", len(set))
	}
}
