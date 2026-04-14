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
			"opencode":       {Enabled: true},
			"claude-code":    {Enabled: false},
			"vscode-copilot": {Enabled: false},
			"cursor":         {Enabled: false},
			"windsurf":       {Enabled: false},
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

// ─── hasRequiredValue tests for new sections ────────────────────────────────

func TestHasRequiredValue_RulesPath(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Required: domain.RequiredBlock{
			Rules: domain.RulesConfig{
				TeamStandards: "standards",
			},
		},
	}
	if !hasRequiredValue(cfg, "rules.team_standards") {
		t.Error("should find required value for rules.team_standards")
	}
}

func TestHasRequiredValue_RulesPath_Empty(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Required: domain.RequiredBlock{},
	}
	if hasRequiredValue(cfg, "rules.team_standards") {
		t.Error("should not find required value when rules is empty")
	}
}

func TestHasRequiredValue_AgentsPath(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Required: domain.RequiredBlock{
			Agents: map[string]domain.AgentDef{
				"reviewer": {Description: "test", Mode: "subagent", Prompt: "test"},
			},
		},
	}
	if !hasRequiredValue(cfg, "agents.reviewer") {
		t.Error("should find required value for agents.reviewer")
	}
	if hasRequiredValue(cfg, "agents.missing") {
		t.Error("should not find required value for agents.missing")
	}
}

func TestHasRequiredValue_MCPPath(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Required: domain.RequiredBlock{
			MCP: map[string]domain.MCPServerDef{
				"context7": {Type: "remote", URL: "https://example.com", Enabled: true},
			},
		},
	}
	if !hasRequiredValue(cfg, "mcp.context7") {
		t.Error("should find required value for mcp.context7")
	}
	if hasRequiredValue(cfg, "mcp.missing") {
		t.Error("should not find required value for mcp.missing")
	}
}

// ─── ValidateProject — new section validation ──────────────────────────────

func TestValidateProject_ValidAgents(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Agents: map[string]domain.AgentDef{
			"reviewer": {Description: "Reviews code", Mode: "subagent", Prompt: "Review this"},
		},
	}
	issues := ValidateProject(cfg)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got: %v", issues)
	}
}

func TestValidateProject_AgentMissingDescription(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Agents: map[string]domain.AgentDef{
			"reviewer": {Mode: "subagent", Prompt: "Review"},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `agent "reviewer" must have a description`)
}

func TestValidateProject_AgentMissingMode(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Agents: map[string]domain.AgentDef{
			"reviewer": {Description: "Code reviewer", Prompt: "Review"},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `agent "reviewer" must have a mode`)
}

func TestValidateProject_AgentUnknownMode(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Agents: map[string]domain.AgentDef{
			"reviewer": {Description: "Code reviewer", Mode: "unknown", Prompt: "Review"},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `unknown mode "unknown"`)
}

func TestValidateProject_AgentMissingPrompt(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Agents: map[string]domain.AgentDef{
			"reviewer": {Description: "Code reviewer", Mode: "subagent"},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `agent "reviewer" must have prompt or prompt_file`)
}

func TestValidateProject_AgentWithPromptFile(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Agents: map[string]domain.AgentDef{
			"reviewer": {Description: "Code reviewer", Mode: "subagent", PromptFile: "prompts/reviewer.md"},
		},
	}
	issues := ValidateProject(cfg)
	if len(issues) != 0 {
		t.Errorf("prompt_file should satisfy prompt requirement, got: %v", issues)
	}
}

func TestValidateProject_ValidSkills(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Skills: map[string]domain.SkillDef{
			"release": {Description: "Generate release notes", Content: "content"},
		},
	}
	issues := ValidateProject(cfg)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got: %v", issues)
	}
}

func TestValidateProject_SkillMissingDescription(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Skills: map[string]domain.SkillDef{
			"release": {Content: "content"},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `skill "release" must have a description`)
}

func TestValidateProject_SkillMissingContent(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Skills: map[string]domain.SkillDef{
			"release": {Description: "Generate release notes"},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `skill "release" must have content or content_file`)
}

func TestValidateProject_ValidCommands(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Commands: map[string]domain.CommandDef{
			"test": {Description: "Run tests"},
		},
	}
	issues := ValidateProject(cfg)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got: %v", issues)
	}
}

func TestValidateProject_CommandMissingDescription(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Commands: map[string]domain.CommandDef{
			"test": {},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `command "test" must have a description`)
}

func TestValidateProject_ValidMCP(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		MCP: map[string]domain.MCPServerDef{
			"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Enabled: true},
			"local-db": {Type: "local", Command: []string{"npx", "server-pg"}, Enabled: true},
		},
	}
	issues := ValidateProject(cfg)
	if len(issues) != 0 {
		t.Errorf("expected no issues, got: %v", issues)
	}
}

func TestValidateProject_MCPMissingType(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		MCP: map[string]domain.MCPServerDef{
			"bad": {URL: "https://example.com", Enabled: true},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `MCP server "bad" must have a type`)
}

func TestValidateProject_MCPUnknownType(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		MCP: map[string]domain.MCPServerDef{
			"bad": {Type: "hybrid", Enabled: true},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `unknown type "hybrid"`)
}

func TestValidateProject_MCPLocalMissingCommand(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		MCP: map[string]domain.MCPServerDef{
			"bad": {Type: "local", Enabled: true},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `(type=local) must have a command`)
}

func TestValidateProject_MCPRemoteMissingURL(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		MCP: map[string]domain.MCPServerDef{
			"bad": {Type: "remote", Enabled: true},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `(type=remote) must have a url`)
}

func TestValidateProject_NewComponentsAccepted(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"memory":   {Enabled: true},
			"rules":    {Enabled: true},
			"settings": {Enabled: true},
			"mcp":      {Enabled: true},
			"agents":   {Enabled: true},
			"skills":   {Enabled: true},
			"commands": {Enabled: true},
		},
	}
	issues := ValidateProject(cfg)
	if len(issues) != 0 {
		t.Errorf("all new component names should be accepted, got: %v", issues)
	}
}

func TestValidateProject_UnknownAdapter(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			"unknown-agent": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `unknown adapter "unknown-agent"`)
}

// ─── ValidatePolicy — new section validation ────────────────────────────────

func TestValidatePolicy_LockedRulesWithRequired(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Version: 1,
		Mode:    domain.ModeTeam,
		Locked:  []string{"rules.team_standards"},
		Required: domain.RequiredBlock{
			Adapters:   map[string]domain.AdapterConfig{},
			Components: map[string]domain.ComponentConfig{},
			Rules:      domain.RulesConfig{TeamStandards: "team rules"},
		},
	}
	issues := ValidatePolicy(cfg)
	for _, issue := range issues {
		if contains(issue, "locked field") {
			t.Errorf("should not report issue for locked rules when required has value, got: %s", issue)
		}
	}
}

func TestValidatePolicy_LockedAgentWithRequired(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Version: 1,
		Mode:    domain.ModeTeam,
		Locked:  []string{"agents.reviewer"},
		Required: domain.RequiredBlock{
			Adapters:   map[string]domain.AdapterConfig{},
			Components: map[string]domain.ComponentConfig{},
			Agents: map[string]domain.AgentDef{
				"reviewer": {Description: "test", Mode: "subagent", Prompt: "test"},
			},
		},
	}
	issues := ValidatePolicy(cfg)
	for _, issue := range issues {
		if contains(issue, "locked field") {
			t.Errorf("should not report issue for locked agent when required has value, got: %s", issue)
		}
	}
}

func TestValidatePolicy_LockedMCPWithRequired(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Version: 1,
		Mode:    domain.ModeTeam,
		Locked:  []string{"mcp.context7"},
		Required: domain.RequiredBlock{
			Adapters:   map[string]domain.AdapterConfig{},
			Components: map[string]domain.ComponentConfig{},
			MCP: map[string]domain.MCPServerDef{
				"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Enabled: true},
			},
		},
	}
	issues := ValidatePolicy(cfg)
	for _, issue := range issues {
		if contains(issue, "locked field") {
			t.Errorf("should not report issue for locked mcp when required has value, got: %s", issue)
		}
	}
}

func TestValidatePolicy_InvalidAgentInRequired(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Version: 1,
		Mode:    domain.ModeTeam,
		Required: domain.RequiredBlock{
			Adapters:   map[string]domain.AdapterConfig{},
			Components: map[string]domain.ComponentConfig{},
			Agents: map[string]domain.AgentDef{
				"bad-agent": {}, // missing description, mode, prompt
			},
		},
	}
	issues := ValidatePolicy(cfg)
	assertContainsIssue(t, issues, `agent "bad-agent" must have a description`)
	assertContainsIssue(t, issues, `agent "bad-agent" must have a mode`)
	assertContainsIssue(t, issues, `agent "bad-agent" must have prompt or prompt_file`)
}

func TestValidatePolicy_InvalidMCPInRequired(t *testing.T) {
	cfg := &domain.PolicyConfig{
		Version: 1,
		Mode:    domain.ModeTeam,
		Required: domain.RequiredBlock{
			Adapters:   map[string]domain.AdapterConfig{},
			Components: map[string]domain.ComponentConfig{},
			MCP: map[string]domain.MCPServerDef{
				"bad-server": {Enabled: true}, // missing type
			},
		},
	}
	issues := ValidatePolicy(cfg)
	assertContainsIssue(t, issues, `MCP server "bad-server" must have a type`)
}

// ─── V2 component acceptance tests ─────────────────────────────────────────

func TestValidateUser_PluginsComponent_Accepted(t *testing.T) {
	cfg := domain.DefaultUserConfig()
	cfg.Components[string(domain.ComponentPlugins)] = domain.ComponentConfig{Enabled: true}
	issues := ValidateUser(cfg)
	for _, issue := range issues {
		if contains(issue, `unknown component "plugins"`) {
			t.Errorf(`"plugins" should be a known component, got issue: %s`, issue)
		}
	}
}

func TestValidateUser_WorkflowsComponent_Accepted(t *testing.T) {
	cfg := domain.DefaultUserConfig()
	cfg.Components[string(domain.ComponentWorkflows)] = domain.ComponentConfig{Enabled: true}
	issues := ValidateUser(cfg)
	for _, issue := range issues {
		if contains(issue, `unknown component "workflows"`) {
			t.Errorf(`"workflows" should be a known component, got issue: %s`, issue)
		}
	}
}

func TestValidateProject_PluginsComponent_Accepted(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"plugins": {Enabled: true},
		},
	}
	issues := ValidateProject(cfg)
	for _, issue := range issues {
		if contains(issue, `unknown component "plugins"`) {
			t.Errorf(`"plugins" should be a known component, got issue: %s`, issue)
		}
	}
}

func TestValidateProject_WorkflowsComponent_Accepted(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			"workflows": {Enabled: true},
		},
	}
	issues := ValidateProject(cfg)
	for _, issue := range issues {
		if contains(issue, `unknown component "workflows"`) {
			t.Errorf(`"workflows" should be a known component, got issue: %s`, issue)
		}
	}
}

func TestValidatePolicy_PluginsComponent_Accepted(t *testing.T) {
	cfg := domain.DefaultPolicyConfig()
	cfg.Required.Components[string(domain.ComponentPlugins)] = domain.ComponentConfig{Enabled: true}
	issues := ValidatePolicy(cfg)
	for _, issue := range issues {
		if contains(issue, `unknown component "plugins"`) {
			t.Errorf(`"plugins" should be a known component in required block, got issue: %s`, issue)
		}
	}
}

func TestValidatePolicy_WorkflowsComponent_Accepted(t *testing.T) {
	cfg := domain.DefaultPolicyConfig()
	cfg.Required.Components[string(domain.ComponentWorkflows)] = domain.ComponentConfig{Enabled: true}
	issues := ValidatePolicy(cfg)
	for _, issue := range issues {
		if contains(issue, `unknown component "workflows"`) {
			t.Errorf(`"workflows" should be a known component in required block, got issue: %s`, issue)
		}
	}
}

func TestValidateProject_AllNineV2Components_Accepted(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version: 1,
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory):    {Enabled: true},
			string(domain.ComponentRules):     {Enabled: true},
			string(domain.ComponentSettings):  {Enabled: true},
			string(domain.ComponentMCP):       {Enabled: true},
			string(domain.ComponentAgents):    {Enabled: true},
			string(domain.ComponentSkills):    {Enabled: true},
			string(domain.ComponentCommands):  {Enabled: true},
			string(domain.ComponentPlugins):   {Enabled: true},
			string(domain.ComponentWorkflows): {Enabled: true},
		},
	}
	issues := ValidateProject(cfg)
	for _, issue := range issues {
		if contains(issue, "unknown component") {
			t.Errorf("all 9 V2 components should be known, got issue: %s", issue)
		}
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

// ─── Methodology validation ─────────────────────────────────────────────────

func TestValidateProject_ValidMethodology(t *testing.T) {
	for _, m := range []domain.Methodology{domain.MethodologyTDD, domain.MethodologySDD, domain.MethodologyConventional} {
		cfg := &domain.ProjectConfig{
			Version:     1,
			Components:  map[string]domain.ComponentConfig{"memory": {Enabled: true}},
			Methodology: m,
		}
		issues := ValidateProject(cfg)
		if len(issues) != 0 {
			t.Errorf("methodology %q should be valid, got: %v", m, issues)
		}
	}
}

func TestValidateProject_EmptyMethodology_NoIssue(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version:    1,
		Components: map[string]domain.ComponentConfig{"memory": {Enabled: true}},
	}
	issues := ValidateProject(cfg)
	if len(issues) != 0 {
		t.Errorf("empty methodology should be valid, got: %v", issues)
	}
}

func TestValidateProject_UnknownMethodology(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version:     1,
		Components:  map[string]domain.ComponentConfig{"memory": {Enabled: true}},
		Methodology: "waterfall",
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `unknown methodology "waterfall"`)
}

// ─── Team role validation ───────────────────────────────────────────────────

func TestValidateProject_ValidTeamRoles(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version:    1,
		Components: map[string]domain.ComponentConfig{"memory": {Enabled: true}},
		Team: map[string]domain.TeamRole{
			"orchestrator": {Description: "TDD orchestrator", Mode: "subagent"},
			"implementer":  {Description: "Implements code", Mode: "inline"},
		},
	}
	issues := ValidateProject(cfg)
	if len(issues) != 0 {
		t.Errorf("valid team roles should pass, got: %v", issues)
	}
}

func TestValidateProject_TeamRole_MissingMode(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version:    1,
		Components: map[string]domain.ComponentConfig{"memory": {Enabled: true}},
		Team: map[string]domain.TeamRole{
			"reviewer": {Description: "Reviews code"},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `team role "reviewer" must have a mode`)
}

func TestValidateProject_TeamRole_UnknownMode(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version:    1,
		Components: map[string]domain.ComponentConfig{"memory": {Enabled: true}},
		Team: map[string]domain.TeamRole{
			"reviewer": {Description: "Reviews code", Mode: "autonomous"},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `unknown mode "autonomous"`)
}

// ─── Plugin validation ──────────────────────────────────────────────────────

func TestValidateProject_ValidPlugins(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version:    1,
		Components: map[string]domain.ComponentConfig{"memory": {Enabled: true}},
		Plugins: map[string]domain.PluginDef{
			"superpowers": {
				Description:   "Cross-agent skills",
				Enabled:       true,
				InstallMethod: "claude_plugin",
			},
		},
	}
	issues := ValidateProject(cfg)
	if len(issues) != 0 {
		t.Errorf("valid plugins should pass, got: %v", issues)
	}
}

func TestValidateProject_Plugin_MissingInstallMethod(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version:    1,
		Components: map[string]domain.ComponentConfig{"memory": {Enabled: true}},
		Plugins: map[string]domain.PluginDef{
			"bad-plugin": {Description: "Bad", Enabled: true},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `plugin "bad-plugin" must have an install_method`)
}

func TestValidateProject_Plugin_UnknownInstallMethod(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version:    1,
		Components: map[string]domain.ComponentConfig{"memory": {Enabled: true}},
		Plugins: map[string]domain.PluginDef{
			"bad-plugin": {Description: "Bad", Enabled: true, InstallMethod: "magic"},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `unknown install_method "magic"`)
}

func TestValidateProject_Plugin_ExcludesMethodology(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version:     1,
		Components:  map[string]domain.ComponentConfig{"memory": {Enabled: true}},
		Methodology: domain.MethodologyTDD,
		Plugins: map[string]domain.PluginDef{
			"superpowers": {
				Description:         "Cross-agent skills",
				Enabled:             true,
				InstallMethod:       "claude_plugin",
				ExcludesMethodology: "tdd",
			},
		},
	}
	issues := ValidateProject(cfg)
	assertContainsIssue(t, issues, `plugin "superpowers" is incompatible with methodology "tdd"`)
}

func TestValidateProject_Plugin_ExcludesMethodology_DisabledPlugin_NoIssue(t *testing.T) {
	cfg := &domain.ProjectConfig{
		Version:     1,
		Components:  map[string]domain.ComponentConfig{"memory": {Enabled: true}},
		Methodology: domain.MethodologyTDD,
		Plugins: map[string]domain.PluginDef{
			"superpowers": {
				Description:         "Cross-agent skills",
				Enabled:             false,
				InstallMethod:       "claude_plugin",
				ExcludesMethodology: "tdd",
			},
		},
	}
	issues := ValidateProject(cfg)
	// Disabled plugin should not trigger methodology exclusion.
	for _, issue := range issues {
		if contains(issue, "incompatible") {
			t.Errorf("disabled plugin should not trigger exclusion, got: %s", issue)
		}
	}
}
