package domain

import (
	"testing"
)

// ─── DefaultUserConfig tests ────────────────────────────────────────────────

func TestDefaultUserConfig_HasCorrectVersion(t *testing.T) {
	cfg := DefaultUserConfig()
	if cfg.Version != 1 {
		t.Errorf("Version = %d, want 1", cfg.Version)
	}
}

func TestDefaultUserConfig_HasHybridMode(t *testing.T) {
	cfg := DefaultUserConfig()
	if cfg.Mode != ModeHybrid {
		t.Errorf("Mode = %q, want %q", cfg.Mode, ModeHybrid)
	}
}

func TestDefaultUserConfig_HasOpenCodeEnabled(t *testing.T) {
	cfg := DefaultUserConfig()
	ac, ok := cfg.Adapters[string(AgentOpenCode)]
	if !ok {
		t.Fatal("opencode adapter not found in defaults")
	}
	if !ac.Enabled {
		t.Error("opencode should be enabled by default")
	}
}

func TestDefaultUserConfig_HasClaudeCodeDisabled(t *testing.T) {
	cfg := DefaultUserConfig()
	ac, ok := cfg.Adapters[string(AgentClaudeCode)]
	if !ok {
		t.Fatal("claude-code adapter not found in defaults")
	}
	if ac.Enabled {
		t.Error("claude-code should be disabled by default")
	}
}

func TestDefaultUserConfig_HasCodexDisabled(t *testing.T) {
	cfg := DefaultUserConfig()
	ac, ok := cfg.Adapters[string(AgentCodex)]
	if !ok {
		t.Fatal("codex adapter not found in defaults")
	}
	if ac.Enabled {
		t.Error("codex should be disabled by default")
	}
}

func TestDefaultUserConfig_HasMemoryEnabled(t *testing.T) {
	cfg := DefaultUserConfig()
	cc, ok := cfg.Components[string(ComponentMemory)]
	if !ok {
		t.Fatal("memory component not found in defaults")
	}
	if !cc.Enabled {
		t.Error("memory should be enabled by default")
	}
}

func TestDefaultUserConfig_HasDefaultBackupDir(t *testing.T) {
	cfg := DefaultUserConfig()
	if cfg.Paths.BackupDir != "~/.agent-manager/backups" {
		t.Errorf("BackupDir = %q, want %q", cfg.Paths.BackupDir, "~/.agent-manager/backups")
	}
}

// ─── DefaultProjectConfig tests ─────────────────────────────────────────────

func TestDefaultProjectConfig_HasCorrectVersion(t *testing.T) {
	cfg := DefaultProjectConfig()
	if cfg.Version != 1 {
		t.Errorf("Version = %d, want 1", cfg.Version)
	}
}

func TestDefaultProjectConfig_HasMemoryEnabled(t *testing.T) {
	cfg := DefaultProjectConfig()
	cc, ok := cfg.Components[string(ComponentMemory)]
	if !ok {
		t.Fatal("memory component not found in defaults")
	}
	if !cc.Enabled {
		t.Error("memory should be enabled by default")
	}
}

func TestDefaultProjectConfig_HasStandardInstructionsTemplate(t *testing.T) {
	cfg := DefaultProjectConfig()
	if cfg.Copilot.InstructionsTemplate != "standard" {
		t.Errorf("InstructionsTemplate = %q, want %q", cfg.Copilot.InstructionsTemplate, "standard")
	}
}

// ─── DefaultPolicyConfig tests ──────────────────────────────────────────────

func TestDefaultPolicyConfig_HasCorrectVersion(t *testing.T) {
	cfg := DefaultPolicyConfig()
	if cfg.Version != 1 {
		t.Errorf("Version = %d, want 1", cfg.Version)
	}
}

func TestDefaultPolicyConfig_HasTeamMode(t *testing.T) {
	cfg := DefaultPolicyConfig()
	if cfg.Mode != ModeTeam {
		t.Errorf("Mode = %q, want %q", cfg.Mode, ModeTeam)
	}
}

func TestDefaultPolicyConfig_HasThreeLockedFields(t *testing.T) {
	cfg := DefaultPolicyConfig()
	if len(cfg.Locked) != 3 {
		t.Errorf("Locked count = %d, want 3", len(cfg.Locked))
	}
}

func TestDefaultPolicyConfig_LocksOpenCode(t *testing.T) {
	cfg := DefaultPolicyConfig()
	found := false
	for _, f := range cfg.Locked {
		if f == "adapters.opencode.enabled" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected adapters.opencode.enabled in locked fields")
	}
}

func TestDefaultPolicyConfig_RequiresOpenCodeEnabled(t *testing.T) {
	cfg := DefaultPolicyConfig()
	ac, ok := cfg.Required.Adapters[string(AgentOpenCode)]
	if !ok {
		t.Fatal("opencode not found in required adapters")
	}
	if !ac.Enabled {
		t.Error("opencode should be required enabled")
	}
}

func TestDefaultPolicyConfig_RequiresMemoryEnabled(t *testing.T) {
	cfg := DefaultPolicyConfig()
	cc, ok := cfg.Required.Components[string(ComponentMemory)]
	if !ok {
		t.Fatal("memory not found in required components")
	}
	if !cc.Enabled {
		t.Error("memory should be required enabled")
	}
}

func TestDefaultPolicyConfig_RequiresStandardCopilotTemplate(t *testing.T) {
	cfg := DefaultPolicyConfig()
	if cfg.Required.Copilot.InstructionsTemplate != "standard" {
		t.Errorf("InstructionsTemplate = %q, want %q",
			cfg.Required.Copilot.InstructionsTemplate, "standard")
	}
}

// ─── Error types tests ──────────────────────────────────────────────────────

func TestPolicyViolationError_Message(t *testing.T) {
	err := &PolicyViolationError{
		Field:          "adapters.opencode.enabled",
		PolicyValue:    "true",
		AttemptedValue: "false",
	}
	msg := err.Error()
	if msg == "" {
		t.Fatal("error message should not be empty")
	}
	if !stringContains(msg, "adapters.opencode.enabled") {
		t.Errorf("error message should contain field name, got: %s", msg)
	}
}

func TestPolicyViolationError_Unwrap(t *testing.T) {
	err := &PolicyViolationError{}
	if err.Unwrap() != ErrPolicyViolation {
		t.Errorf("Unwrap = %v, want ErrPolicyViolation", err.Unwrap())
	}
}

func TestValidationError_Message(t *testing.T) {
	err := &ValidationError{
		Source: "config.json",
		Issues: []string{"bad field", "missing value"},
	}
	msg := err.Error()
	if !stringContains(msg, "config.json") {
		t.Errorf("error message should contain source, got: %s", msg)
	}
	if !stringContains(msg, "2 issue") {
		t.Errorf("error message should contain issue count, got: %s", msg)
	}
}

func TestValidationError_Unwrap(t *testing.T) {
	err := &ValidationError{Source: "test", Issues: []string{"x"}}
	if err.Unwrap() != ErrInvalidConfig {
		t.Errorf("Unwrap = %v, want ErrInvalidConfig", err.Unwrap())
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
