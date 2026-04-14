package claude

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// Adapter implements domain.Adapter for Claude Code.
// Claude Code is a personal-lane agent — enabled per-user, not by policy.
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
	return domain.AgentClaudeCode
}

// Lane returns the adapter lane. Claude Code is personal-optional.
func (a *Adapter) Lane() domain.AdapterLane {
	return domain.LanePersonal
}

// Detect checks whether the claude binary is on PATH and whether
// the config directory (~/.claude) exists.
func (a *Adapter) Detect(_ context.Context, homeDir string) (installed bool, configFound bool, err error) {
	_, lookErr := a.lookPath("claude")
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

// GlobalConfigDir returns ~/.claude.
func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return ConfigDir(homeDir)
}

// SystemPromptFile returns ~/.claude/CLAUDE.md.
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "CLAUDE.md")
}

// SkillsDir returns ~/.claude/skills.
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "skills")
}

// SettingsPath returns ~/.claude/settings.json.
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "settings.json")
}

// SupportsComponent reports whether Claude Code supports a given component.
func (a *Adapter) SupportsComponent(c domain.ComponentID) bool {
	switch c {
	case domain.ComponentMemory, domain.ComponentRules, domain.ComponentSettings,
		domain.ComponentSkills, domain.ComponentMCP, domain.ComponentPlugins:
		return true
	default:
		return false
	}
}

// ProjectConfigFile returns <projectDir>/.claude/settings.json.
func (a *Adapter) ProjectConfigFile(projectDir string) string {
	return filepath.Join(projectDir, ".claude", "settings.json")
}

// ProjectRulesFile returns <projectDir>/CLAUDE.md.
func (a *Adapter) ProjectRulesFile(projectDir string) string {
	return filepath.Join(projectDir, "CLAUDE.md")
}

// ProjectAgentsDir returns empty string — Claude Code does not support project agents.
func (a *Adapter) ProjectAgentsDir(_ string) string {
	return ""
}

// ProjectSkillsDir returns <projectDir>/.claude/skills.
func (a *Adapter) ProjectSkillsDir(projectDir string) string {
	return filepath.Join(projectDir, ".claude", "skills")
}

// ProjectCommandsDir returns empty string — Claude Code does not support project commands.
func (a *Adapter) ProjectCommandsDir(_ string) string {
	return ""
}

// ProjectSettingsPath returns <projectDir>/.claude/settings.json.
func (a *Adapter) ProjectSettingsPath(projectDir string) string {
	return filepath.Join(projectDir, ".claude", "settings.json")
}

// ConfigDir returns the root config directory for Claude Code.
func ConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude")
}

// MCPDir returns the directory for per-server MCP configuration files.
// Claude Code uses separate files per MCP server: ~/.claude/mcp/{name}.json
func (a *Adapter) MCPDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "mcp")
}

// DelegationStrategy returns DelegationPromptBased — Claude Code uses Task tool delegation.
func (a *Adapter) DelegationStrategy() domain.DelegationStrategy {
	return domain.DelegationPromptBased
}

// SupportsSubAgents returns false — Claude Code does not create named sub-agent files.
func (a *Adapter) SupportsSubAgents() bool {
	return false
}

// SubAgentsDir returns empty string — Claude Code does not support sub-agent files.
func (a *Adapter) SubAgentsDir(_ string) string {
	return ""
}

// SupportsWorkflows returns false — Claude Code does not support workflow files.
func (a *Adapter) SupportsWorkflows() bool {
	return false
}

// WorkflowsDir returns empty string — Claude Code does not support workflows.
func (a *Adapter) WorkflowsDir(_ string) string {
	return ""
}
