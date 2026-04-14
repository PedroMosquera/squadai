package domain

import (
	"encoding/json"
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

func TestDefaultUserConfig_HasTwoAdapters(t *testing.T) {
	cfg := DefaultUserConfig()
	if len(cfg.Adapters) != 2 {
		t.Errorf("Adapters count = %d, want 2 (opencode, claude-code)", len(cfg.Adapters))
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
	if cfg.Paths.BackupDir != "~/.squadai/backups" {
		t.Errorf("BackupDir = %q, want %q", cfg.Paths.BackupDir, "~/.squadai/backups")
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

// ─── New ComponentID tests ──────────────────────────────────────────────────

func TestComponentIDs_AllDefined(t *testing.T) {
	ids := []ComponentID{
		ComponentMemory,
		ComponentRules,
		ComponentSettings,
		ComponentMCP,
		ComponentAgents,
		ComponentSkills,
		ComponentCommands,
		ComponentPlugins,
		ComponentWorkflows,
	}
	for _, id := range ids {
		if id == "" {
			t.Error("ComponentID should not be empty")
		}
	}
	if len(ids) != 9 {
		t.Errorf("expected 9 component IDs, got %d", len(ids))
	}
}

func TestComponentPlugins_HasCorrectValue(t *testing.T) {
	if ComponentPlugins != "plugins" {
		t.Errorf("ComponentPlugins = %q, want %q", ComponentPlugins, "plugins")
	}
}

func TestComponentWorkflows_HasCorrectValue(t *testing.T) {
	if ComponentWorkflows != "workflows" {
		t.Errorf("ComponentWorkflows = %q, want %q", ComponentWorkflows, "workflows")
	}
}

// ─── Rich AdapterConfig tests ──────────────────────────────────────────────

func TestAdapterConfig_WithSettings(t *testing.T) {
	ac := AdapterConfig{
		Enabled: true,
		Settings: map[string]interface{}{
			"model":      "anthropic/claude-sonnet-4-5",
			"autoupdate": true,
		},
	}
	if !ac.Enabled {
		t.Error("adapter should be enabled")
	}
	if ac.Settings["model"] != "anthropic/claude-sonnet-4-5" {
		t.Errorf("model = %v, want anthropic/claude-sonnet-4-5", ac.Settings["model"])
	}
	if ac.Settings["autoupdate"] != true {
		t.Errorf("autoupdate = %v, want true", ac.Settings["autoupdate"])
	}
}

func TestAdapterConfig_NilSettings(t *testing.T) {
	ac := AdapterConfig{Enabled: true}
	if ac.Settings != nil {
		t.Error("settings should be nil by default")
	}
}

// ─── Rich ComponentConfig tests ────────────────────────────────────────────

func TestComponentConfig_WithSettings(t *testing.T) {
	cc := ComponentConfig{
		Enabled: true,
		Settings: map[string]interface{}{
			"depth": float64(5),
		},
	}
	if !cc.Enabled {
		t.Error("component should be enabled")
	}
	if cc.Settings["depth"] != float64(5) {
		t.Errorf("depth = %v, want 5", cc.Settings["depth"])
	}
}

// ─── CopilotConfig tests ───────────────────────────────────────────────────

func TestCopilotConfig_CustomContent(t *testing.T) {
	cc := CopilotConfig{
		InstructionsTemplate: "custom",
		CustomContent:        "## My Custom Instructions",
	}
	if cc.InstructionsTemplate != "custom" {
		t.Errorf("InstructionsTemplate = %q, want custom", cc.InstructionsTemplate)
	}
	if cc.CustomContent != "## My Custom Instructions" {
		t.Errorf("CustomContent = %q, want ## My Custom Instructions", cc.CustomContent)
	}
}

// ─── New type tests ────────────────────────────────────────────────────────

func TestRulesConfig_Fields(t *testing.T) {
	rc := RulesConfig{
		TeamStandards:     "## Standards",
		TeamStandardsFile: "templates/team.md",
		Instructions:      []string{"extra.md"},
	}
	if rc.TeamStandards != "## Standards" {
		t.Error("TeamStandards mismatch")
	}
	if rc.TeamStandardsFile != "templates/team.md" {
		t.Error("TeamStandardsFile mismatch")
	}
	if len(rc.Instructions) != 1 {
		t.Error("Instructions count mismatch")
	}
}

func TestAgentDef_Fields(t *testing.T) {
	ad := AgentDef{
		Description: "A code reviewer",
		Mode:        "subagent",
		Model:       "anthropic/claude-sonnet-4-5",
		Prompt:      "Review this code",
		Permission:  map[string]string{"edit": "deny"},
	}
	if ad.Description != "A code reviewer" {
		t.Error("Description mismatch")
	}
	if ad.Mode != "subagent" {
		t.Error("Mode mismatch")
	}
	if ad.Permission["edit"] != "deny" {
		t.Error("Permission mismatch")
	}
}

func TestSkillDef_Fields(t *testing.T) {
	sd := SkillDef{
		Description: "Generate release notes",
		Content:     "## Release Notes\nGenerate...",
	}
	if sd.Description != "Generate release notes" {
		t.Error("Description mismatch")
	}
}

func TestCommandDef_Fields(t *testing.T) {
	cd := CommandDef{
		Description: "Run all tests",
		Template:    "Run go test ./...",
		Agent:       "code-reviewer",
	}
	if cd.Description != "Run all tests" {
		t.Error("Description mismatch")
	}
}

func TestMCPServerDef_Local(t *testing.T) {
	mcp := MCPServerDef{
		Type:    "local",
		Command: []string{"npx", "-y", "@modelcontextprotocol/server-postgres"},
		Enabled: true,
		Environment: map[string]string{
			"DATABASE_URL": "postgres://localhost:5432/db",
		},
	}
	if mcp.Type != "local" {
		t.Error("Type mismatch")
	}
	if len(mcp.Command) != 3 {
		t.Errorf("Command length = %d, want 3", len(mcp.Command))
	}
	if !mcp.Enabled {
		t.Error("should be enabled")
	}
}

func TestMCPServerDef_Remote(t *testing.T) {
	mcp := MCPServerDef{
		Type:    "remote",
		URL:     "https://mcp.context7.com/mcp",
		Enabled: true,
	}
	if mcp.Type != "remote" {
		t.Error("Type mismatch")
	}
	if mcp.URL != "https://mcp.context7.com/mcp" {
		t.Error("URL mismatch")
	}
}

func TestProjectMeta_Fields(t *testing.T) {
	pm := ProjectMeta{
		Name:         "my-project",
		Language:     "Go",
		Framework:    "bubbletea",
		TestCommand:  "go test ./...",
		BuildCommand: "go build ./...",
		LintCommand:  "golangci-lint run ./...",
	}
	if pm.Name != "my-project" {
		t.Error("Name mismatch")
	}
	if pm.Language != "Go" {
		t.Error("Language mismatch")
	}
}

// ─── JSON serialization tests ──────────────────────────────────────────────

func TestAdapterConfig_JSONRoundTrip(t *testing.T) {
	ac := AdapterConfig{
		Enabled: true,
		Settings: map[string]interface{}{
			"model": "test-model",
		},
	}

	data, err := json.Marshal(ac)
	if err != nil {
		t.Fatal(err)
	}

	var decoded AdapterConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if !decoded.Enabled {
		t.Error("decoded Enabled should be true")
	}
	if decoded.Settings["model"] != "test-model" {
		t.Errorf("decoded model = %v, want test-model", decoded.Settings["model"])
	}
}

func TestProjectConfig_JSONRoundTrip(t *testing.T) {
	pc := ProjectConfig{
		Version: 1,
		Adapters: map[string]AdapterConfig{
			"opencode": {
				Enabled: true,
				Settings: map[string]interface{}{
					"model": "test",
				},
			},
		},
		Components: map[string]ComponentConfig{
			"memory": {Enabled: true},
		},
		Copilot: CopilotConfig{
			InstructionsTemplate: "custom",
			CustomContent:        "my content",
		},
		Rules: RulesConfig{
			TeamStandards: "## Standards",
		},
		Agents: map[string]AgentDef{
			"reviewer": {Description: "Reviews code", Mode: "subagent", Prompt: "Review"},
		},
		MCP: map[string]MCPServerDef{
			"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Enabled: true},
		},
		Meta: ProjectMeta{Name: "test-proj", Language: "Go"},
	}

	data, err := json.Marshal(pc)
	if err != nil {
		t.Fatal(err)
	}

	var decoded ProjectConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Version != 1 {
		t.Errorf("Version = %d, want 1", decoded.Version)
	}
	if decoded.Copilot.CustomContent != "my content" {
		t.Error("CustomContent mismatch after round-trip")
	}
	if decoded.Rules.TeamStandards != "## Standards" {
		t.Error("TeamStandards mismatch after round-trip")
	}
	if _, ok := decoded.Agents["reviewer"]; !ok {
		t.Error("reviewer agent missing after round-trip")
	}
	if _, ok := decoded.MCP["context7"]; !ok {
		t.Error("context7 MCP server missing after round-trip")
	}
	if decoded.Meta.Name != "test-proj" {
		t.Error("Meta.Name mismatch after round-trip")
	}
	if decoded.Adapters["opencode"].Settings["model"] != "test" {
		t.Error("adapter settings model mismatch after round-trip")
	}
}

func TestAdapterConfig_JSONOmitsNilSettings(t *testing.T) {
	ac := AdapterConfig{Enabled: true}

	data, err := json.Marshal(ac)
	if err != nil {
		t.Fatal(err)
	}

	// JSON should not contain "settings" when nil
	if stringContains(string(data), "settings") {
		t.Errorf("JSON should omit nil settings, got: %s", string(data))
	}
}

func TestComponentConfig_JSONOmitsNilSettings(t *testing.T) {
	cc := ComponentConfig{Enabled: true}

	data, err := json.Marshal(cc)
	if err != nil {
		t.Fatal(err)
	}

	if stringContains(string(data), "settings") {
		t.Errorf("JSON should omit nil settings, got: %s", string(data))
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

// ─── V2 domain type tests ──────────────────────────────────────────────────

func TestAgentIDs_AllV2Defined(t *testing.T) {
	ids := []AgentID{
		AgentOpenCode,
		AgentClaudeCode,
		AgentVSCodeCopilot,
		AgentCursor,
		AgentWindsurf,
	}
	for _, id := range ids {
		if id == "" {
			t.Error("AgentID should not be empty")
		}
	}
	if len(ids) != 5 {
		t.Errorf("expected 5 agent IDs, got %d", len(ids))
	}
}

func TestMethodology_AllDefined(t *testing.T) {
	methodologies := []Methodology{
		MethodologyTDD,
		MethodologySDD,
		MethodologyConventional,
	}
	for _, m := range methodologies {
		if m == "" {
			t.Error("Methodology should not be empty")
		}
	}
	if len(methodologies) != 3 {
		t.Errorf("expected 3 methodologies, got %d", len(methodologies))
	}
}

func TestDelegationStrategy_AllDefined(t *testing.T) {
	strategies := []DelegationStrategy{
		DelegationNativeAgents,
		DelegationPromptBased,
		DelegationSoloAgent,
	}
	for _, s := range strategies {
		if s == "" {
			t.Error("DelegationStrategy should not be empty")
		}
	}
	if len(strategies) != 3 {
		t.Errorf("expected 3 delegation strategies, got %d", len(strategies))
	}
}

func TestPluginDef_Fields(t *testing.T) {
	pd := PluginDef{
		Description:         "Cross-agent skills framework",
		Enabled:             true,
		SupportedAgents:     []string{"claude-code", "opencode", "cursor"},
		InstallMethod:       "claude_plugin",
		PluginID:            "superpowers@claude-plugins-official",
		ExcludesMethodology: "tdd",
	}
	if pd.Description != "Cross-agent skills framework" {
		t.Error("Description mismatch")
	}
	if !pd.Enabled {
		t.Error("should be enabled")
	}
	if len(pd.SupportedAgents) != 3 {
		t.Errorf("SupportedAgents count = %d, want 3", len(pd.SupportedAgents))
	}
	if pd.InstallMethod != "claude_plugin" {
		t.Error("InstallMethod mismatch")
	}
	if pd.ExcludesMethodology != "tdd" {
		t.Error("ExcludesMethodology mismatch")
	}
}

func TestTeamRole_Fields(t *testing.T) {
	tr := TeamRole{
		Description: "TDD orchestrator",
		Mode:        "subagent",
		SkillRef:    "tdd/brainstorming",
		DelegatesTo: []string{"planner", "implementer"},
	}
	if tr.Description != "TDD orchestrator" {
		t.Error("Description mismatch")
	}
	if tr.Mode != "subagent" {
		t.Error("Mode mismatch")
	}
	if tr.SkillRef != "tdd/brainstorming" {
		t.Error("SkillRef mismatch")
	}
	if len(tr.DelegatesTo) != 2 {
		t.Errorf("DelegatesTo count = %d, want 2", len(tr.DelegatesTo))
	}
}

func TestPluginDef_JSONRoundTrip(t *testing.T) {
	pd := PluginDef{
		Description:     "Test plugin",
		Enabled:         true,
		SupportedAgents: []string{"opencode"},
		InstallMethod:   "skill_files",
	}

	data, err := json.Marshal(pd)
	if err != nil {
		t.Fatal(err)
	}

	var decoded PluginDef
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Description != "Test plugin" {
		t.Error("Description mismatch after round-trip")
	}
	if !decoded.Enabled {
		t.Error("Enabled mismatch after round-trip")
	}
	if len(decoded.SupportedAgents) != 1 {
		t.Error("SupportedAgents mismatch after round-trip")
	}
}

func TestTeamRole_JSONRoundTrip(t *testing.T) {
	tr := TeamRole{
		Description: "Test role",
		Mode:        "inline",
		SkillRef:    "shared/testing",
	}

	data, err := json.Marshal(tr)
	if err != nil {
		t.Fatal(err)
	}

	var decoded TeamRole
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Description != "Test role" {
		t.Error("Description mismatch after round-trip")
	}
	if decoded.Mode != "inline" {
		t.Error("Mode mismatch after round-trip")
	}
}
