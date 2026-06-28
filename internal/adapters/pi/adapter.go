package pi

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// Adapter implements domain.Adapter for the Pi agent.
// Pi is a personal-lane adapter with native sub-agent delegation.
type Adapter struct {
	// lookPath resolves a binary name to an absolute path.
	// Defaults to exec.LookPath. Injected for testing.
	lookPath func(name string) (string, error)

	// statPath checks whether a filesystem path exists.
	// Defaults to os.Stat. Injected for testing.
	statPath func(name string) (os.FileInfo, error)
}

// New returns an Adapter with production filesystem dependencies.
func New() *Adapter {
	return &Adapter{
		lookPath: exec.LookPath,
		statPath: os.Stat,
	}
}

// NewWithDeps returns an Adapter with injected dependencies (for testing).
func NewWithDeps(lookPath func(string) (string, error), statPath func(string) (os.FileInfo, error)) *Adapter {
	return &Adapter{
		lookPath: lookPath,
		statPath: statPath,
	}
}

// ID returns the agent identifier.
func (a *Adapter) ID() domain.AgentID {
	return domain.AgentPi
}

// Lane returns the adapter lane. Pi is a personal-optional adapter.
func (a *Adapter) Lane() domain.AdapterLane {
	return domain.LanePersonal
}

// Detect checks whether the pi binary is on PATH and whether
// the config directory (~/.pi/agent) exists.
func (a *Adapter) Detect(_ context.Context, homeDir string) (installed bool, configFound bool, err error) {
	_, lookErr := a.lookPath("pi")
	if lookErr == nil {
		installed = true
	}

	configDir := ConfigDir(homeDir)
	info, statErr := a.statPath(configDir)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			return installed, false, nil
		}
		return false, false, statErr
	}
	configFound = info.IsDir()

	return installed, configFound, nil
}

// GlobalConfigDir returns ~/.pi/agent.
func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return ConfigDir(homeDir)
}

// SystemPromptFile returns ~/.pi/agent/AGENTS.md.
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "AGENTS.md")
}

// SkillsDir returns ~/.pi/agent/skills.
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "skills")
}

// SettingsPath returns ~/.pi/agent/settings.json.
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "settings.json")
}

// SupportsComponent reports whether Pi supports a given component.
func (a *Adapter) SupportsComponent(c domain.ComponentID) bool {
	switch c {
	case domain.ComponentMemory, domain.ComponentRules, domain.ComponentSettings,
		domain.ComponentMCP, domain.ComponentAgents, domain.ComponentSkills,
		domain.ComponentCommands, domain.ComponentPlugins, domain.ComponentPermissions,
		domain.ComponentBrand:
		return true
	default:
		return false
	}
}

// ProjectConfigFile returns <projectDir>/pi.json.
func (a *Adapter) ProjectConfigFile(projectDir string) string {
	return filepath.Join(projectDir, "pi.json")
}

// ProjectRulesFile returns <projectDir>/AGENTS.md.
func (a *Adapter) ProjectRulesFile(projectDir string) string {
	return filepath.Join(projectDir, "AGENTS.md")
}

// ProjectAgentsDir returns <projectDir>/.pi/agents.
func (a *Adapter) ProjectAgentsDir(projectDir string) string {
	return filepath.Join(projectDir, ".pi", "agents")
}

// ProjectSkillsDir returns <projectDir>/.pi/skills.
func (a *Adapter) ProjectSkillsDir(projectDir string) string {
	return filepath.Join(projectDir, ".pi", "skills")
}

// ProjectCommandsDir returns <projectDir>/.pi/prompts — Pi renders commands as prompt templates.
func (a *Adapter) ProjectCommandsDir(projectDir string) string {
	return filepath.Join(projectDir, ".pi", "prompts")
}

// AgentsDir returns ~/.pi/agent/agents.
func (a *Adapter) AgentsDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "agents")
}

// CommandsDir returns ~/.pi/agent/prompts — Pi's prompt templates serve as commands.
func (a *Adapter) CommandsDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "prompts")
}

// PromptsDir returns ~/.pi/agent/prompts. This is a Pi-specific method not part of
// the domain.Adapter interface; callers that need it will type-assert.
func (a *Adapter) PromptsDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "prompts")
}

// ConfigDir returns the root config directory for Pi.
func ConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".pi", "agent")
}

// DelegationStrategy returns DelegationNativeAgents — Pi supports native sub-agents.
func (a *Adapter) DelegationStrategy() domain.DelegationStrategy {
	return domain.DelegationNativeAgents
}

// SupportsSubAgents returns true — Pi supports named sub-agents.
func (a *Adapter) SupportsSubAgents() bool {
	return true
}

// SubAgentsDir returns ~/.pi/agent/agents.
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "agents")
}

// SupportsWorkflows returns false — Pi does not support workflow files.
func (a *Adapter) SupportsWorkflows() bool {
	return false
}

// WorkflowsDir returns empty string — Pi does not support workflows.
func (a *Adapter) WorkflowsDir(_ string) string {
	return ""
}

// MCPRootKey returns "mcp" — Pi merges MCP servers under the "mcp" key.
func (a *Adapter) MCPRootKey() string { return "mcp" }

// MCPURLKey returns "url" — Pi uses the standard URL key.
func (a *Adapter) MCPURLKey() string { return "url" }

// MCPConfigPath returns empty string — Pi uses MergeIntoSettings (no separate MCP file).
func (a *Adapter) MCPConfigPath(_ string) string { return "" }

// MCPCommandStyle returns "array" — Pi encodes the full command in a single array.
func (a *Adapter) MCPCommandStyle() string { return "array" }

// MCPEnvKey returns "env" — Pi uses the standard env key.
func (a *Adapter) MCPEnvKey() string { return "env" }

// MCPTypeField echoes def.Type — Pi always emits the type field for both stdio and remote.
func (a *Adapter) MCPTypeField(def domain.MCPServerDef) string { return def.Type }

// RulesFrontmatter returns empty string — Pi uses marker-based injection.
func (a *Adapter) RulesFrontmatter() string { return "" }

// RulesFileSizeCap returns 0 — Pi has no known rules file size limit.
func (a *Adapter) RulesFileSizeCap() int { return 0 }
