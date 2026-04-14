package domain

// UserConfig represents ~/.squadai/config.json.
type UserConfig struct {
	Version    int                        `json:"version"`
	Mode       OperationalMode            `json:"mode"`
	Adapters   map[string]AdapterConfig   `json:"adapters"`
	Components map[string]ComponentConfig `json:"components"`
	Paths      PathsConfig                `json:"paths"`
}

// ProjectConfig represents .squadai/project.json.
type ProjectConfig struct {
	Version     int                        `json:"version"`
	Adapters    map[string]AdapterConfig   `json:"adapters,omitempty"`
	Components  map[string]ComponentConfig `json:"components"`
	Copilot     CopilotConfig              `json:"copilot"`
	Rules       RulesConfig                `json:"rules,omitempty"`
	Agents      map[string]AgentDef        `json:"agents,omitempty"`
	Skills      map[string]SkillDef        `json:"skills,omitempty"`
	Commands    map[string]CommandDef      `json:"commands,omitempty"`
	MCP         map[string]MCPServerDef    `json:"mcp,omitempty"`
	Meta        ProjectMeta                `json:"meta,omitempty"`
	Methodology Methodology                `json:"methodology,omitempty"`
	ModelTier   ModelTier                  `json:"model_tier,omitempty"`
	Team        map[string]TeamRole        `json:"team,omitempty"`
	Plugins     map[string]PluginDef       `json:"plugins,omitempty"`
}

// PolicyConfig represents .squadai/policy.json.
type PolicyConfig struct {
	Version  int             `json:"version"`
	Mode     OperationalMode `json:"mode"`
	Locked   []string        `json:"locked"`
	Required RequiredBlock   `json:"required"`
}

// RequiredBlock defines the enforced values in a policy.
type RequiredBlock struct {
	Adapters   map[string]AdapterConfig   `json:"adapters"`
	Components map[string]ComponentConfig `json:"components"`
	Copilot    CopilotConfig              `json:"copilot"`
	Rules      RulesConfig                `json:"rules,omitempty"`
	Agents     map[string]AgentDef        `json:"agents,omitempty"`
	MCP        map[string]MCPServerDef    `json:"mcp,omitempty"`
	Plugins    map[string]PluginDef       `json:"plugins,omitempty"`
}

// AdapterConfig is the per-adapter configuration in config files.
type AdapterConfig struct {
	Enabled  bool                   `json:"enabled"`
	Settings map[string]interface{} `json:"settings,omitempty"`
}

// ComponentConfig is the per-component configuration in config files.
type ComponentConfig struct {
	Enabled  bool                   `json:"enabled"`
	Settings map[string]interface{} `json:"settings,omitempty"`
}

// CopilotConfig controls shared copilot-instructions behavior.
type CopilotConfig struct {
	InstructionsTemplate string `json:"instructions_template"`
	CustomContent        string `json:"custom_content,omitempty"`

	// Meta is populated by the merger from ProjectConfig.Meta. It is not
	// serialized because the canonical source is ProjectConfig.Meta.
	Meta ProjectMeta `json:"-"`
}

// RulesConfig defines team standards content for instruction files.
type RulesConfig struct {
	// TeamStandards is inline markdown content injected into AGENTS.md / CLAUDE.md.
	TeamStandards string `json:"team_standards,omitempty"`
	// TeamStandardsFile is a path (relative to .squadai/) to load content from.
	TeamStandardsFile string `json:"team_standards_file,omitempty"`
	// Instructions lists additional instruction file paths to reference.
	Instructions []string `json:"instructions,omitempty"`
}

// PluginDef defines a third-party plugin that can be installed for supported agents.
type PluginDef struct {
	Description         string   `json:"description"`
	Enabled             bool     `json:"enabled"`
	SupportedAgents     []string `json:"supported_agents"`
	InstallMethod       string   `json:"install_method"`                 // "claude_plugin" or "skill_files"
	PluginID            string   `json:"plugin_id,omitempty"`            // e.g., "superpowers@claude-plugins-official"
	ExcludesMethodology string   `json:"excludes_methodology,omitempty"` // blocked when this methodology is active
}

// TeamRole defines a role within a methodology team.
type TeamRole struct {
	Description string   `json:"description"`
	Mode        string   `json:"mode"`                   // "subagent" or "inline"
	SkillRef    string   `json:"skill_ref,omitempty"`    // e.g., "tdd/brainstorming"
	DelegatesTo []string `json:"delegates_to,omitempty"` // roles this role can delegate to
}

// AgentDef defines a custom agent for OpenCode's .opencode/agents/ directory.
type AgentDef struct {
	Description string            `json:"description"`
	Mode        string            `json:"mode"`
	Model       string            `json:"model,omitempty"`
	Prompt      string            `json:"prompt,omitempty"`
	PromptFile  string            `json:"prompt_file,omitempty"`
	Permission  map[string]string `json:"permission,omitempty"`
}

// SkillDef defines a custom skill for OpenCode's .opencode/skills/ directory.
type SkillDef struct {
	Description string `json:"description"`
	Content     string `json:"content,omitempty"`
	ContentFile string `json:"content_file,omitempty"`
}

// CommandDef defines a custom command for OpenCode's .opencode/commands/ directory.
type CommandDef struct {
	Description string `json:"description"`
	Template    string `json:"template,omitempty"`
	Agent       string `json:"agent,omitempty"`
	Model       string `json:"model,omitempty"`
}

// MCPServerDef defines an MCP server configuration.
type MCPServerDef struct {
	Type        string            `json:"type"`
	Command     []string          `json:"command,omitempty"`
	URL         string            `json:"url,omitempty"`
	Enabled     bool              `json:"enabled"`
	Environment map[string]string `json:"environment,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
}

// ProjectMeta holds project metadata used for template rendering.
type ProjectMeta struct {
	Name           string   `json:"name,omitempty"`
	Language       string   `json:"language,omitempty"`
	Languages      []string `json:"languages,omitempty"`
	Framework      string   `json:"framework,omitempty"`
	TestCommand    string   `json:"test_command,omitempty"`
	BuildCommand   string   `json:"build_command,omitempty"`
	LintCommand    string   `json:"lint_command,omitempty"`
	PackageManager string   `json:"package_manager,omitempty"`
}

// PathsConfig holds user-overridable paths.
type PathsConfig struct {
	BackupDir string `json:"backup_dir"`
}

// MergedConfig is the resolved configuration after applying precedence rules.
// Policy locked fields are final. Project overrides user. User provides defaults.
type MergedConfig struct {
	Mode        OperationalMode            `json:"mode"`
	Adapters    map[string]AdapterConfig   `json:"adapters"`
	Components  map[string]ComponentConfig `json:"components"`
	Copilot     CopilotConfig              `json:"copilot"`
	Rules       RulesConfig                `json:"rules,omitempty"`
	Agents      map[string]AgentDef        `json:"agents,omitempty"`
	Skills      map[string]SkillDef        `json:"skills,omitempty"`
	Commands    map[string]CommandDef      `json:"commands,omitempty"`
	MCP         map[string]MCPServerDef    `json:"mcp,omitempty"`
	Meta        ProjectMeta                `json:"meta,omitempty"`
	Paths       PathsConfig                `json:"paths"`
	Methodology Methodology                `json:"methodology,omitempty"`
	ModelTier   ModelTier                  `json:"model_tier,omitempty"`
	Team        map[string]TeamRole        `json:"team,omitempty"`
	Plugins     map[string]PluginDef       `json:"plugins,omitempty"`

	// Violations is populated during merge when user/project values conflicted
	// with locked policy fields. These are informational — the policy value wins.
	Violations []string `json:"violations,omitempty"`
}

// DefaultUserConfig returns a sensible default for first-time users.
func DefaultUserConfig() *UserConfig {
	return &UserConfig{
		Version: 1,
		Mode:    ModeHybrid,
		Adapters: map[string]AdapterConfig{
			string(AgentOpenCode):   {Enabled: true},
			string(AgentClaudeCode): {Enabled: false},
		},
		Components: map[string]ComponentConfig{
			string(ComponentMemory): {Enabled: true},
		},
		Paths: PathsConfig{
			BackupDir: "~/.squadai/backups",
		},
	}
}

// DefaultProjectConfig returns a minimal project config.
func DefaultProjectConfig() *ProjectConfig {
	return &ProjectConfig{
		Version: 1,
		Components: map[string]ComponentConfig{
			string(ComponentMemory): {Enabled: true},
		},
		Copilot: CopilotConfig{
			InstructionsTemplate: "standard",
		},
	}
}

// DefaultTeam returns the default team composition for the given methodology.
// Each role has a description, mode ("subagent"), and a SkillRef pointing to
// the embedded skill asset. Returns nil for unknown methodologies.
func DefaultTeam(m Methodology) map[string]TeamRole {
	switch m {
	case MethodologyTDD:
		return map[string]TeamRole{
			"orchestrator": {Description: "TDD orchestrator — delegates phases to specialized sub-agents", Mode: "subagent"},
			"brainstormer": {Description: "Question-asking and requirements exploration", Mode: "subagent", SkillRef: "tdd/brainstorming"},
			"planner":      {Description: "Test plan and implementation plan creation", Mode: "subagent", SkillRef: "tdd/writing-plans"},
			"implementer":  {Description: "Red-green-refactor implementation cycles", Mode: "subagent", SkillRef: "tdd/test-driven-development"},
			"reviewer":     {Description: "Two-stage code review: automated + design", Mode: "subagent", SkillRef: "shared/code-review"},
			"debugger":     {Description: "4-phase debugging: reproduce → isolate → fix → verify", Mode: "subagent", SkillRef: "tdd/systematic-debugging"},
		}
	case MethodologySDD:
		return map[string]TeamRole{
			"orchestrator": {Description: "SDD orchestrator — manages spec-driven workflow", Mode: "subagent"},
			"explorer":     {Description: "Codebase analysis and context gathering", Mode: "subagent", SkillRef: "sdd/sdd-explore"},
			"proposer":     {Description: "Solution proposals with tradeoff analysis", Mode: "subagent", SkillRef: "sdd/sdd-propose"},
			"spec-writer":  {Description: "Formal specification document authoring", Mode: "subagent", SkillRef: "sdd/sdd-spec"},
			"designer":     {Description: "Architecture and interface design", Mode: "subagent", SkillRef: "sdd/sdd-design"},
			"task-planner": {Description: "Dependency-ordered task breakdown", Mode: "subagent", SkillRef: "sdd/sdd-tasks"},
			"implementer":  {Description: "Spec-faithful implementation", Mode: "subagent", SkillRef: "sdd/sdd-apply"},
			"verifier":     {Description: "Spec compliance verification", Mode: "subagent", SkillRef: "sdd/sdd-verify"},
		}
	case MethodologyConventional:
		return map[string]TeamRole{
			"orchestrator": {Description: "Conventional orchestrator — direct implementation with review gates", Mode: "subagent"},
			"implementer":  {Description: "General-purpose implementation", Mode: "subagent"},
			"reviewer":     {Description: "Code review checklist", Mode: "subagent", SkillRef: "shared/code-review"},
			"tester":       {Description: "Test writing and coverage", Mode: "subagent", SkillRef: "shared/testing"},
		}
	}
	return nil
}

// DefaultPolicyConfig returns a team policy that locks the baseline.
func DefaultPolicyConfig() *PolicyConfig {
	return &PolicyConfig{
		Version: 1,
		Mode:    ModeTeam,
		Locked: []string{
			"adapters.opencode.enabled",
			"components.memory.enabled",
			"copilot.instructions_template",
		},
		Required: RequiredBlock{
			Adapters: map[string]AdapterConfig{
				string(AgentOpenCode): {Enabled: true},
			},
			Components: map[string]ComponentConfig{
				string(ComponentMemory): {Enabled: true},
			},
			Copilot: CopilotConfig{
				InstructionsTemplate: "standard",
			},
		},
	}
}
