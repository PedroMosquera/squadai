package windsurf

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// Adapter implements domain.Adapter for the Windsurf agent.
// Windsurf is a personal-lane, solo-delegation agent — the only adapter that supports workflows.
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
	return domain.AgentWindsurf
}

// Lane returns the adapter lane. Windsurf is personal-optional.
func (a *Adapter) Lane() domain.AdapterLane {
	return domain.LanePersonal
}

// Detect checks whether the windsurf binary is on PATH and whether
// the config directory (~/.codeium/windsurf) exists.
func (a *Adapter) Detect(_ context.Context, homeDir string) (installed bool, configFound bool, err error) {
	_, lookErr := a.lookPath("windsurf")
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

// GlobalConfigDir returns ~/.codeium/windsurf.
func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return ConfigDir(homeDir)
}

// SystemPromptFile returns ~/.codeium/windsurf/global_rules.md.
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "global_rules.md")
}

// SkillsDir returns ~/.codeium/windsurf/skills.
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "skills")
}

// SettingsPath returns ~/.codeium/windsurf/mcp_config.json.
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "mcp_config.json")
}

// SupportsComponent reports whether Windsurf supports a given component.
func (a *Adapter) SupportsComponent(c domain.ComponentID) bool {
	switch c {
	case domain.ComponentMemory, domain.ComponentRules, domain.ComponentSettings,
		domain.ComponentMCP, domain.ComponentSkills, domain.ComponentPlugins,
		domain.ComponentWorkflows:
		return true
	default:
		return false
	}
}

// ProjectConfigFile returns <projectDir>/.windsurf/mcp_config.json.
func (a *Adapter) ProjectConfigFile(projectDir string) string {
	return filepath.Join(projectDir, ".windsurf", "mcp_config.json")
}

// ProjectRulesFile returns <projectDir>/.windsurfrules.
func (a *Adapter) ProjectRulesFile(projectDir string) string {
	return filepath.Join(projectDir, ".windsurfrules")
}

// ProjectAgentsDir returns empty string — Windsurf is a solo agent with no sub-agents.
func (a *Adapter) ProjectAgentsDir(_ string) string {
	return ""
}

// ProjectSkillsDir returns <projectDir>/.windsurf/skills.
func (a *Adapter) ProjectSkillsDir(projectDir string) string {
	return filepath.Join(projectDir, ".windsurf", "skills")
}

// ProjectCommandsDir returns empty string — Windsurf does not support commands.
func (a *Adapter) ProjectCommandsDir(_ string) string {
	return ""
}

// DelegationStrategy returns DelegationSoloAgent — Windsurf runs all phases inline.
func (a *Adapter) DelegationStrategy() domain.DelegationStrategy {
	return domain.DelegationSoloAgent
}

// SupportsSubAgents returns false — Windsurf is a solo agent.
func (a *Adapter) SupportsSubAgents() bool {
	return false
}

// SubAgentsDir returns empty string — Windsurf does not support sub-agents.
func (a *Adapter) SubAgentsDir(_ string) string {
	return ""
}

// SupportsWorkflows returns true — Windsurf is the only adapter that supports workflow files.
func (a *Adapter) SupportsWorkflows() bool {
	return true
}

// WorkflowsDir returns <projectDir>/.windsurf/workflows.
func (a *Adapter) WorkflowsDir(projectDir string) string {
	return filepath.Join(projectDir, ".windsurf", "workflows")
}

// ConfigDir returns the root config directory for Windsurf.
func ConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".codeium", "windsurf")
}
