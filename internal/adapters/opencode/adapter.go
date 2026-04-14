package opencode

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// Adapter implements domain.Adapter for the OpenCode agent.
// OpenCode is the team-baseline engine — always available in team/hybrid modes.
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
	return domain.AgentOpenCode
}

// Lane returns the adapter lane. OpenCode is always team-required.
func (a *Adapter) Lane() domain.AdapterLane {
	return domain.LaneTeam
}

// Detect checks whether the opencode binary is on PATH and whether
// the config directory (~/.config/opencode) exists.
func (a *Adapter) Detect(_ context.Context, homeDir string) (installed bool, configFound bool, err error) {
	_, lookErr := a.lookPath("opencode")
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

// GlobalConfigDir returns ~/.config/opencode.
func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return ConfigDir(homeDir)
}

// SystemPromptFile returns ~/.config/opencode/AGENTS.md.
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "AGENTS.md")
}

// SkillsDir returns ~/.config/opencode/skills.
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "skills")
}

// SettingsPath returns ~/.config/opencode/opencode.json.
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "opencode.json")
}

// SupportsComponent reports whether OpenCode supports a given component.
func (a *Adapter) SupportsComponent(c domain.ComponentID) bool {
	switch c {
	case domain.ComponentMemory, domain.ComponentRules, domain.ComponentSettings,
		domain.ComponentMCP, domain.ComponentAgents, domain.ComponentSkills,
		domain.ComponentCommands, domain.ComponentPlugins:
		return true
	default:
		return false
	}
}

// ProjectConfigFile returns <projectDir>/opencode.json.
func (a *Adapter) ProjectConfigFile(projectDir string) string {
	return filepath.Join(projectDir, "opencode.json")
}

// ProjectRulesFile returns <projectDir>/AGENTS.md.
func (a *Adapter) ProjectRulesFile(projectDir string) string {
	return filepath.Join(projectDir, "AGENTS.md")
}

// ProjectAgentsDir returns <projectDir>/.opencode/agents.
func (a *Adapter) ProjectAgentsDir(projectDir string) string {
	return filepath.Join(projectDir, ".opencode", "agents")
}

// ProjectSkillsDir returns <projectDir>/.opencode/skills.
func (a *Adapter) ProjectSkillsDir(projectDir string) string {
	return filepath.Join(projectDir, ".opencode", "skills")
}

// ProjectCommandsDir returns <projectDir>/.opencode/commands.
func (a *Adapter) ProjectCommandsDir(projectDir string) string {
	return filepath.Join(projectDir, ".opencode", "commands")
}

// AgentsDir returns ~/.config/opencode/agents.
func (a *Adapter) AgentsDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "agents")
}

// CommandsDir returns ~/.config/opencode/commands.
func (a *Adapter) CommandsDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "commands")
}

// ConfigDir returns the root config directory for OpenCode.
func ConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".config", "opencode")
}

// DelegationStrategy returns DelegationNativeAgents — OpenCode supports native sub-agents.
func (a *Adapter) DelegationStrategy() domain.DelegationStrategy {
	return domain.DelegationNativeAgents
}

// SupportsSubAgents returns true — OpenCode supports named sub-agents.
func (a *Adapter) SupportsSubAgents() bool {
	return true
}

// SubAgentsDir returns ~/.config/opencode/agents.
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "agents")
}

// SupportsWorkflows returns false — OpenCode does not support workflow files.
func (a *Adapter) SupportsWorkflows() bool {
	return false
}

// WorkflowsDir returns empty string — OpenCode does not support workflows.
func (a *Adapter) WorkflowsDir(_ string) string {
	return ""
}
