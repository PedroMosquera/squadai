package domain

// UserConfig represents ~/.agent-manager/config.json.
type UserConfig struct {
	Version    int                      `json:"version"`
	Mode       OperationalMode          `json:"mode"`
	Adapters   map[string]AdapterConfig `json:"adapters"`
	Components map[string]ComponentConfig `json:"components"`
	Paths      PathsConfig              `json:"paths"`
}

// ProjectConfig represents .agent-manager/project.json.
type ProjectConfig struct {
	Version    int                        `json:"version"`
	Components map[string]ComponentConfig `json:"components"`
	Copilot    CopilotConfig              `json:"copilot"`
}

// PolicyConfig represents .agent-manager/policy.json.
type PolicyConfig struct {
	Version  int              `json:"version"`
	Mode     OperationalMode  `json:"mode"`
	Locked   []string         `json:"locked"`
	Required RequiredBlock    `json:"required"`
}

// RequiredBlock defines the enforced values in a policy.
type RequiredBlock struct {
	Adapters   map[string]AdapterConfig   `json:"adapters"`
	Components map[string]ComponentConfig `json:"components"`
	Copilot    CopilotConfig              `json:"copilot"`
}

// AdapterConfig is the per-adapter toggle in config files.
type AdapterConfig struct {
	Enabled bool `json:"enabled"`
}

// ComponentConfig is the per-component toggle in config files.
type ComponentConfig struct {
	Enabled bool `json:"enabled"`
}

// CopilotConfig controls shared copilot-instructions behavior.
type CopilotConfig struct {
	InstructionsTemplate string `json:"instructions_template"`
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
