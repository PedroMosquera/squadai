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

// ─── Deep settings merge tests ──────────────────────────────────────────────

func TestMerge_AdapterSettings_UserCarriedThrough(t *testing.T) {
	user := &domain.UserConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {
				Enabled: true,
				Settings: map[string]interface{}{
					"model":      "anthropic/claude-sonnet-4-5",
					"autoupdate": true,
				},
			},
		},
	}

	merged := Merge(user, nil, nil)

	ac := merged.Adapters["opencode"]
	if ac.Settings["model"] != "anthropic/claude-sonnet-4-5" {
		t.Errorf("model = %v, want anthropic/claude-sonnet-4-5", ac.Settings["model"])
	}
	if ac.Settings["autoupdate"] != true {
		t.Errorf("autoupdate = %v, want true", ac.Settings["autoupdate"])
	}
}

func TestMerge_AdapterSettings_ProjectOverridesUser(t *testing.T) {
	user := &domain.UserConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {
				Enabled: true,
				Settings: map[string]interface{}{
					"model":      "anthropic/claude-sonnet-4-5",
					"autoupdate": true,
				},
			},
		},
	}
	project := &domain.ProjectConfig{
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {
				Enabled: true,
				Settings: map[string]interface{}{
					"model": "anthropic/claude-opus-4",
				},
			},
		},
	}

	merged := Merge(user, project, nil)

	ac := merged.Adapters["opencode"]
	// model overridden by project
	if ac.Settings["model"] != "anthropic/claude-opus-4" {
		t.Errorf("model = %v, want anthropic/claude-opus-4", ac.Settings["model"])
	}
	// autoupdate preserved from user
	if ac.Settings["autoupdate"] != true {
		t.Errorf("autoupdate = %v, want true (preserved from user)", ac.Settings["autoupdate"])
	}
}

func TestMerge_AdapterSettings_PolicyOverridesAll(t *testing.T) {
	user := &domain.UserConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {
				Enabled: true,
				Settings: map[string]interface{}{
					"model":      "user-model",
					"autoupdate": true,
				},
			},
		},
	}
	policy := &domain.PolicyConfig{
		Mode:   domain.ModeTeam,
		Locked: []string{"adapters.opencode.settings.model"},
		Required: domain.RequiredBlock{
			Adapters: map[string]domain.AdapterConfig{
				"opencode": {
					Enabled: true,
					Settings: map[string]interface{}{
						"model": "policy-model",
					},
				},
			},
		},
	}

	merged := Merge(user, nil, policy)

	ac := merged.Adapters["opencode"]
	if ac.Settings["model"] != "policy-model" {
		t.Errorf("model = %v, want policy-model", ac.Settings["model"])
	}
	// autoupdate preserved from user (not in policy)
	if ac.Settings["autoupdate"] != true {
		t.Errorf("autoupdate = %v, want true (preserved from user)", ac.Settings["autoupdate"])
	}
	// Should record a violation for locked settings field
	if len(merged.Violations) == 0 {
		t.Fatal("expected violation for locked settings field")
	}
	found := false
	for _, v := range merged.Violations {
		if strings.Contains(v, "adapters.opencode.settings.model") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected violation about adapters.opencode.settings.model, got: %v", merged.Violations)
	}
}

func TestMerge_ComponentSettings_DeepMerge(t *testing.T) {
	user := &domain.UserConfig{
		Mode: domain.ModePersonal,
		Components: map[string]domain.ComponentConfig{
			"memory": {
				Enabled: true,
				Settings: map[string]interface{}{
					"depth": float64(10),
				},
			},
		},
	}
	project := &domain.ProjectConfig{
		Components: map[string]domain.ComponentConfig{
			"memory": {
				Enabled: true,
				Settings: map[string]interface{}{
					"format": "markdown",
				},
			},
		},
	}

	merged := Merge(user, project, nil)

	cc := merged.Components["memory"]
	if cc.Settings["depth"] != float64(10) {
		t.Errorf("depth = %v, want 10 (preserved from user)", cc.Settings["depth"])
	}
	if cc.Settings["format"] != "markdown" {
		t.Errorf("format = %v, want markdown (from project)", cc.Settings["format"])
	}
}

func TestMerge_NilSettings_NoNilPointerPanic(t *testing.T) {
	user := &domain.UserConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true}, // nil settings
		},
	}
	project := &domain.ProjectConfig{
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {
				Enabled: true,
				Settings: map[string]interface{}{
					"model": "test",
				},
			},
		},
	}

	// Should not panic
	merged := Merge(user, project, nil)

	if merged.Adapters["opencode"].Settings["model"] != "test" {
		t.Error("settings should be populated from project even when user has nil settings")
	}
}

// ─── New config sections merge tests ────────────────────────────────────────

func TestMerge_ProjectRules_CarriedThrough(t *testing.T) {
	project := &domain.ProjectConfig{
		Version: 1,
		Rules: domain.RulesConfig{
			TeamStandards: "## Team Standards\n\n- Use Go 1.24+",
		},
	}

	merged := Merge(nil, project, nil)

	if merged.Rules.TeamStandards != "## Team Standards\n\n- Use Go 1.24+" {
		t.Errorf("TeamStandards = %q, want expected content", merged.Rules.TeamStandards)
	}
}

func TestMerge_PolicyRules_OverrideProject(t *testing.T) {
	project := &domain.ProjectConfig{
		Version: 1,
		Rules: domain.RulesConfig{
			TeamStandards: "project standards",
		},
	}
	policy := &domain.PolicyConfig{
		Mode:   domain.ModeTeam,
		Locked: []string{"rules.team_standards"},
		Required: domain.RequiredBlock{
			Rules: domain.RulesConfig{
				TeamStandards: "policy standards",
			},
		},
	}

	merged := Merge(nil, project, policy)

	if merged.Rules.TeamStandards != "policy standards" {
		t.Errorf("TeamStandards = %q, want 'policy standards'", merged.Rules.TeamStandards)
	}
	if len(merged.Violations) == 0 {
		t.Fatal("expected violation for locked rules.team_standards")
	}
}

func TestMerge_ProjectAgents_CarriedThrough(t *testing.T) {
	project := &domain.ProjectConfig{
		Version: 1,
		Agents: map[string]domain.AgentDef{
			"reviewer": {Description: "Code reviewer", Mode: "subagent", Prompt: "Review"},
		},
	}

	merged := Merge(nil, project, nil)

	if len(merged.Agents) != 1 {
		t.Fatalf("Agents count = %d, want 1", len(merged.Agents))
	}
	if merged.Agents["reviewer"].Description != "Code reviewer" {
		t.Error("reviewer agent description mismatch")
	}
}

func TestMerge_PolicyAgents_Override(t *testing.T) {
	project := &domain.ProjectConfig{
		Version: 1,
		Agents: map[string]domain.AgentDef{
			"reviewer": {Description: "Project reviewer", Mode: "subagent", Prompt: "Review"},
		},
	}
	policy := &domain.PolicyConfig{
		Mode:   domain.ModeTeam,
		Locked: []string{"agents.reviewer"},
		Required: domain.RequiredBlock{
			Agents: map[string]domain.AgentDef{
				"reviewer": {Description: "Policy reviewer", Mode: "subagent", Prompt: "Review per policy"},
			},
		},
	}

	merged := Merge(nil, project, policy)

	if merged.Agents["reviewer"].Description != "Policy reviewer" {
		t.Errorf("reviewer Description = %q, want 'Policy reviewer'", merged.Agents["reviewer"].Description)
	}
	if len(merged.Violations) == 0 {
		t.Fatal("expected violation for locked agents.reviewer")
	}
}

func TestMerge_ProjectMCP_CarriedThrough(t *testing.T) {
	project := &domain.ProjectConfig{
		Version: 1,
		MCP: map[string]domain.MCPServerDef{
			"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Enabled: true},
		},
	}

	merged := Merge(nil, project, nil)

	if len(merged.MCP) != 1 {
		t.Fatalf("MCP count = %d, want 1", len(merged.MCP))
	}
	if merged.MCP["context7"].URL != "https://mcp.context7.com/mcp" {
		t.Error("context7 URL mismatch")
	}
}

func TestMerge_PolicyMCP_Override(t *testing.T) {
	project := &domain.ProjectConfig{
		Version: 1,
		MCP: map[string]domain.MCPServerDef{
			"sentry": {Type: "remote", URL: "https://project-url.com", Enabled: true},
		},
	}
	policy := &domain.PolicyConfig{
		Mode:   domain.ModeTeam,
		Locked: []string{"mcp.sentry"},
		Required: domain.RequiredBlock{
			MCP: map[string]domain.MCPServerDef{
				"sentry": {Type: "remote", URL: "https://policy-url.com", Enabled: true},
			},
		},
	}

	merged := Merge(nil, project, policy)

	if merged.MCP["sentry"].URL != "https://policy-url.com" {
		t.Errorf("sentry URL = %q, want policy URL", merged.MCP["sentry"].URL)
	}
	if len(merged.Violations) == 0 {
		t.Fatal("expected violation for locked mcp.sentry")
	}
}

func TestMerge_ProjectSkillsAndCommands_CarriedThrough(t *testing.T) {
	project := &domain.ProjectConfig{
		Version: 1,
		Skills: map[string]domain.SkillDef{
			"release": {Description: "Generate release notes", Content: "content"},
		},
		Commands: map[string]domain.CommandDef{
			"test": {Description: "Run tests"},
		},
	}

	merged := Merge(nil, project, nil)

	if len(merged.Skills) != 1 {
		t.Fatalf("Skills count = %d, want 1", len(merged.Skills))
	}
	if len(merged.Commands) != 1 {
		t.Fatalf("Commands count = %d, want 1", len(merged.Commands))
	}
}

func TestMerge_ProjectMeta_CarriedThrough(t *testing.T) {
	project := &domain.ProjectConfig{
		Version: 1,
		Meta: domain.ProjectMeta{
			Name:     "my-project",
			Language: "Go",
		},
	}

	merged := Merge(nil, project, nil)

	if merged.Meta.Name != "my-project" {
		t.Errorf("Meta.Name = %q, want my-project", merged.Meta.Name)
	}
	if merged.Meta.Language != "Go" {
		t.Errorf("Meta.Language = %q, want Go", merged.Meta.Language)
	}
}

func TestMerge_CopilotCustomContent_CarriedThrough(t *testing.T) {
	project := &domain.ProjectConfig{
		Version: 1,
		Copilot: domain.CopilotConfig{
			InstructionsTemplate: "custom",
			CustomContent:        "## My Custom Instructions",
		},
	}

	merged := Merge(nil, project, nil)

	if merged.Copilot.CustomContent != "## My Custom Instructions" {
		t.Errorf("CustomContent = %q, want '## My Custom Instructions'", merged.Copilot.CustomContent)
	}
}

// ─── Settings isolation tests ───────────────────────────────────────────────

func TestMerge_DoesNotMutateInput_UserAdapterSettings(t *testing.T) {
	user := &domain.UserConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {
				Enabled: true,
				Settings: map[string]interface{}{
					"model": "original",
				},
			},
		},
	}
	project := &domain.ProjectConfig{
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {
				Enabled: true,
				Settings: map[string]interface{}{
					"model": "override",
				},
			},
		},
	}

	_ = Merge(user, project, nil)

	// Original user settings should not be mutated
	if user.Adapters["opencode"].Settings["model"] != "original" {
		t.Error("merge should not mutate input user config settings")
	}
}
