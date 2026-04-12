package config

import (
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// ─── ValidateUser tests ─────────────────────────────────────────────────────

func TestValidateUser_ValidConfig_NoIssues(t *testing.T) {
	cfg := domain.DefaultUserConfig()
	issues := ValidateUser(cfg)
	if len(issues) != 0 {
		t.Errorf("expected no issues for valid config, got: %v", issues)
	}
}

func TestValidateUser_ZeroVersion(t *testing.T) {
	cfg := domain.DefaultUserConfig()
	cfg.Version = 0
	issues := ValidateUser(cfg)
	assertContainsIssue(t, issues, "version must be >= 1")
}

func TestValidateUser_EmptyMode(t *testing.T) {
	cfg := domain.DefaultUserConfig()
	cfg.Mode = ""
	issues := ValidateUser(cfg)
	assertContainsIssue(t, issues, "mode is required")
}

func TestValidateUser_UnknownMode(t *testing.T) {
	cfg := domain.DefaultUserConfig()
	cfg.Mode = "chaos"
	issues := ValidateUser(cfg)
	assertContainsIssue(t, issues, "unknown mode")
}

func TestValidateUser_AllValidModes(t *testing.T) {
	modes := []domain.OperationalMode{domain.ModeTeam, domain.ModePersonal, domain.ModeHybrid}
	for _, m := range modes {
		cfg := domain.DefaultUserConfig()
		cfg.Mode = m
		issues := ValidateUser(cfg)
		for _, issue := range issues {
			if issue == "mode is required" || contains(issue, "unknown mode") {
				t.Errorf("mode %q should be valid, got issue: %s", m, issue)
			}
		}
	}
}

func TestValidateUser_UnknownAdapter(t *testing.T) {
	cfg := domain.DefaultUserConfig()
	cfg.Adapters["unknown-agent"] = domain.AdapterConfig{Enabled: true}
	issues := ValidateUser(cfg)
	assertContainsIssue(t, issues, `unknown adapter "unknown-agent"`)
}

func TestValidateUser_UnknownComponent(t *testing.T) {
	cfg := domain.DefaultUserConfig()
	cfg.Components["unknown-comp"] = domain.ComponentConfig{Enabled: true}
	issues := ValidateUser(cfg)
	assertContainsIssue(t, issues, `unknown component "unknown-comp"`)
}

func TestValidateUser_KnownAdaptersPass(t *testing.T) {
	cfg := &domain.UserConfig{
		Version: 1,
		Mode:    domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"opencode":    {Enabled: true},
			"claude-code": {Enabled: false},
			"codex":       {Enabled: false},
		},
		Components: map[string]domain.ComponentConfig{},
	}
	issues := ValidateUser(cfg)
	if len(issues) != 0 {
		t.Errorf("all known adapters should pass, got: %v", issues)
	}
}

// ─── ValidateProject tests ──────────────────────────────────────────────────

func TestValidateProject_ValidConfig_NoIssues(t *testing.T) {
	cfg := domain.DefaultProjectConfig()
	issues := ValidateProject(cfg)
	if len(issues) != 0 {
		t.Errorf("expected no issues for valid config, got: %v", issues)
	}
}

func TestValidateProject_ZeroVersion(t *testing.T) {
	cfg := domain.DefaultProjectConfig()
	cfg.Version = 0
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, "version must be >= 1")
}

func TestValidateProject_UnknownComponent(t *testing.T) {
	cfg := domain.DefaultProjectConfig()
	cfg.Components["fancy-widget"] = domain.ComponentConfig{Enabled: true}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `unknown component "fancy-widget"`)
}

// ─── ValidatePolicy tests ───────────────────────────────────────────────────

func TestValidatePolicy_ValidConfig_NoIssues(t *testing.T) {
	cfg := domain.DefaultPolicyConfig()
	issues := ValidatePolicy(cfg)
	if len(issues) != 0 {
		t.Errorf("expected no issues for valid config, got: %v", issues)
	}
}

func TestValidatePolicy_ZeroVersion(t *testing.T) {
	cfg := domain.DefaultPolicyConfig()
	cfg.Version = 0
	issues := ValidatePolicy(cfg)
	assertContainsIssue(t, issues, "version must be >= 1")
}

func TestValidatePolicy_EmptyMode(t *testing.T) {
	cfg := domain.DefaultPolicyConfig()
	cfg.Mode = ""
	issues := ValidatePolicy(cfg)
	assertContainsIssue(t, issues, "mode is required")
}

func TestValidatePolicy_UnknownMode(t *testing.T) {
	cfg := domain.DefaultPolicyConfig()
	cfg.Mode = "yolo"
	issues := ValidatePolicy(cfg)
	assertContainsIssue(t, issues, "unknown mode")
}

func TestValidatePolicy_LockedFieldWithoutRequired_ReportsIssue(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Version: 1,
		Mode:    domain.ModeTeam,
		Locked:  []string{"adapters.opencode.enabled"},
		Required: domain.RequiredBlock{
			// No adapter values set — locked field has no corresponding value
			Adapters:   map[string]domain.AdapterConfig{},
			Components: map[string]domain.ComponentConfig{},
		},
	}
	issues := ValidatePolicy(cfg)
	assertContainsIssue(t, issues, "locked field")
	assertContainsIssue(t, issues, "no corresponding value")
}

func TestValidatePolicy_LockedFieldWithRequired_NoIssue(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Version: 1,
		Mode:    domain.ModeTeam,
		Locked:  []string{"adapters.opencode.enabled"},
		Required: domain.RequiredBlock{
			Adapters: map[string]domain.AdapterConfig{
				"opencode": {Enabled: true},
			},
			Components: map[string]domain.ComponentConfig{},
		},
	}
	issues := ValidatePolicy(cfg)
	for _, issue := range issues {
		if contains(issue, "locked field") {
			t.Errorf("should not report locked field issue when required value exists, got: %s", issue)
		}
	}
}

func TestValidatePolicy_UnknownAdapter_InRequired(t *testing.T) {
	cfg := domain.DefaultPolicyConfig()
	cfg.Required.Adapters["ghost-agent"] = domain.AdapterConfig{Enabled: true}
	issues := ValidatePolicy(cfg)
	assertContainsIssue(t, issues, `unknown adapter "ghost-agent" in required block`)
}

func TestValidatePolicy_UnknownComponent_InRequired(t *testing.T) {
	cfg := domain.DefaultPolicyConfig()
	cfg.Required.Components["fantasy"] = domain.ComponentConfig{Enabled: true}
	issues := ValidatePolicy(cfg)
	assertContainsIssue(t, issues, `unknown component "fantasy" in required block`)
}

// ─── hasRequiredValue tests ─────────────────────────────────────────────────

func TestHasRequiredValue_AdapterPath(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Required: domain.RequiredBlock{
			Adapters: map[string]domain.AdapterConfig{
				"opencode": {Enabled: true},
			},
		},
	}
	if !hasRequiredValue(cfg, "adapters.opencode.enabled") {
		t.Error("should find required value for adapters.opencode.enabled")
	}
	if hasRequiredValue(cfg, "adapters.missing.enabled") {
		t.Error("should not find required value for adapters.missing.enabled")
	}
}

func TestHasRequiredValue_ComponentPath(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Required: domain.RequiredBlock{
			Components: map[string]domain.ComponentConfig{
				"memory": {Enabled: true},
			},
		},
	}
	if !hasRequiredValue(cfg, "components.memory.enabled") {
		t.Error("should find required value for components.memory.enabled")
	}
}

func TestHasRequiredValue_CopilotPath(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Required: domain.RequiredBlock{
			Copilot: domain.CopilotConfig{
				InstructionsTemplate: "standard",
			},
		},
	}
	if !hasRequiredValue(cfg, "copilot.instructions_template") {
		t.Error("should find required value for copilot.instructions_template")
	}
}

func TestHasRequiredValue_EmptyCopilot(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Required: domain.RequiredBlock{},
	}
	if hasRequiredValue(cfg, "copilot.instructions_template") {
		t.Error("should not find required value when copilot template is empty")
	}
}

func TestHasRequiredValue_ShortPath(t *testing.T) {
	cfg := &domain.PolicyConfig{}
	if hasRequiredValue(cfg, "single") {
		t.Error("single-segment path should return false")
	}
}

func TestHasRequiredValue_UnknownPrefix(t *testing.T) {
	cfg := &domain.PolicyConfig{}
	if hasRequiredValue(cfg, "unknown.field.value") {
		t.Error("unknown prefix should return false")
	}
}

func TestHasRequiredValue_ShortAdapterPath(t *testing.T) {
	cfg := &domain.PolicyConfig{}
	if hasRequiredValue(cfg, "adapters.opencode") {
		t.Error("two-segment adapter path should return false (needs 3)")
	}
}

func TestHasRequiredValue_ShortComponentPath(t *testing.T) {
	cfg := &domain.PolicyConfig{}
	if hasRequiredValue(cfg, "components.memory") {
		t.Error("two-segment component path should return false (needs 3)")
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func assertContainsIssue(t *testing.T, issues []string, substr string) {
	t.Helper()
	for _, issue := range issues {
		if contains(issue, substr) {
			return
		}
	}
	t.Errorf("expected issue containing %q, got: %v", substr, issues)
}
