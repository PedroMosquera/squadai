package cursor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// Adapter implements domain.Adapter for the Cursor agent.
// Cursor is a personal-lane editor with native sub-agent delegation.
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
	return domain.AgentCursor
}

// Lane returns the adapter lane. Cursor is personal-optional.
func (a *Adapter) Lane() domain.AdapterLane {
	return domain.LanePersonal
}

// Detect checks whether the cursor binary is on PATH and whether
// the config directory (~/.cursor) exists.
func (a *Adapter) Detect(_ context.Context, homeDir string) (installed bool, configFound bool, err error) {
	_, lookErr := a.lookPath("cursor")
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

// GlobalConfigDir returns ~/.cursor.
func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return ConfigDir(homeDir)
}

// SystemPromptFile returns ~/.cursor/.cursorrules.
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), ".cursorrules")
}

// SkillsDir returns ~/.cursor/skills.
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "skills")
}

// SettingsPath returns ~/.cursor/mcp.json.
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "mcp.json")
}

// SupportsComponent reports whether Cursor supports a given component.
func (a *Adapter) SupportsComponent(c domain.ComponentID) bool {
	switch c {
	case domain.ComponentMemory, domain.ComponentRules, domain.ComponentSettings,
		domain.ComponentMCP, domain.ComponentAgents, domain.ComponentSkills:
		return true
	default:
		return false
	}
}

// ProjectConfigFile returns <projectDir>/.cursor/mcp.json.
func (a *Adapter) ProjectConfigFile(projectDir string) string {
	return filepath.Join(projectDir, ".cursor", "mcp.json")
}

// ProjectRulesFile returns <projectDir>/.cursorrules.
func (a *Adapter) ProjectRulesFile(projectDir string) string {
	return filepath.Join(projectDir, ".cursorrules")
}

// ProjectAgentsDir returns <projectDir>/.cursor/agents.
func (a *Adapter) ProjectAgentsDir(projectDir string) string {
	return filepath.Join(projectDir, ".cursor", "agents")
}

// ProjectSkillsDir returns <projectDir>/.cursor/skills.
func (a *Adapter) ProjectSkillsDir(projectDir string) string {
	return filepath.Join(projectDir, ".cursor", "skills")
}

// ProjectCommandsDir returns empty string — Cursor does not support project commands.
func (a *Adapter) ProjectCommandsDir(_ string) string {
	return ""
}

// DelegationStrategy returns DelegationNativeAgents — Cursor supports native sub-agents.
func (a *Adapter) DelegationStrategy() domain.DelegationStrategy {
	return domain.DelegationNativeAgents
}

// SupportsSubAgents returns true — Cursor supports named sub-agents.
func (a *Adapter) SupportsSubAgents() bool {
	return true
}

// SubAgentsDir returns ~/.cursor/agents.
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "agents")
}

// SupportsWorkflows returns false — Cursor does not support workflow files.
func (a *Adapter) SupportsWorkflows() bool {
	return false
}

// WorkflowsDir returns empty string — Cursor does not support workflows.
func (a *Adapter) WorkflowsDir(_ string) string {
	return ""
}

// ConfigDir returns the root config directory for Cursor.
func ConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".cursor")
}
