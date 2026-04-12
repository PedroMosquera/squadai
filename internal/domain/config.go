package domain

// UserConfig represents ~/.agent-manager/config.json.
type UserConfig struct {
	Version    int                        `json:"version"`
	Mode       OperationalMode            `json:"mode"`
	Adapters   map[string]AdapterConfig   `json:"adapters"`
	Components map[string]ComponentConfig `json:"components"`
	Paths      PathsConfig                `json:"paths"`
}

// ProjectConfig represents .agent-manager/project.json.
type ProjectConfig struct {
	Version    int                        `json:"version"`
	Adapters   map[string]AdapterConfig   `json:"adapters,omitempty"`
	Components map[string]ComponentConfig `json:"components"`
	Copilot    CopilotConfig              `json:"copilot"`
	Rules      RulesConfig                `json:"rules,omitempty"`
	Agents     map[string]AgentDef        `json:"agents,omitempty"`
	Skills     map[string]SkillDef        `json:"skills,omitempty"`
	Commands   map[string]CommandDef      `json:"commands,omitempty"`
	MCP        map[string]MCPServerDef    `json:"mcp,omitempty"`
	Meta       ProjectMeta                `json:"meta,omitempty"`
}

// PolicyConfig represents .agent-manager/policy.json.
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
	// TeamStandardsFile is a path (relative to .agent-manager/) to load content from.
	TeamStandardsFile string `json:"team_standards_file,omitempty"`
	// Instructions lists additional instruction file paths to reference.
	Instructions []string `json:"instructions,omitempty"`
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
	Name         string `json:"name,omitempty"`
	Language     string `json:"language,omitempty"`
	Framework    string `json:"framework,omitempty"`
	TestCommand  string `json:"test_command,omitempty"`
	BuildCommand string `json:"build_command,omitempty"`
	LintCommand  string `json:"lint_command,omitempty"`
}

// PathsConfig holds user-overridable paths.
type PathsConfig struct {
	BackupDir string `json:"backup_dir"`
}

// MergedConfig is the resolved configuration after applying precedence rules.
// Policy locked fields are final. Project overrides user. User provides defaults.
type MergedConfig struct {
	Mode       OperationalMode            `json:"mode"`
	Adapters   map[string]AdapterConfig   `json:"adapters"`
	Components map[string]ComponentConfig `json:"components"`
	Copilot    CopilotConfig              `json:"copilot"`
	Rules      RulesConfig                `json:"rules,omitempty"`
	Agents     map[string]AgentDef        `json:"agents,omitempty"`
	Skills     map[string]SkillDef        `json:"skills,omitempty"`
	Commands   map[string]CommandDef      `json:"commands,omitempty"`
	MCP        map[string]MCPServerDef    `json:"mcp,omitempty"`
	Meta       ProjectMeta                `json:"meta,omitempty"`
	Paths      PathsConfig                `json:"paths"`

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
			string(AgentCodex):      {Enabled: false},
		},
		Components: map[string]ComponentConfig{
			string(ComponentMemory): {Enabled: true},
		},
		Paths: PathsConfig{
			BackupDir: "~/.agent-manager/backups",
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
