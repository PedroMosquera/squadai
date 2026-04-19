package claude

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/PedroMosquera/squadai/internal/domain"
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
		domain.ComponentSkills, domain.ComponentMCP, domain.ComponentPlugins,
		domain.ComponentAgents, domain.ComponentPermissions:
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

// ProjectAgentsDir returns <projectDir>/.claude/agents — Claude Code native sub-agent directory.
func (a *Adapter) ProjectAgentsDir(projectDir string) string {
	return filepath.Join(projectDir, ".claude", "agents")
}

// ProjectSkillsDir returns <projectDir>/.claude/skills.
func (a *Adapter) ProjectSkillsDir(projectDir string) string {
	return filepath.Join(projectDir, ".claude", "skills")
}

// ProjectCommandsDir returns empty string — Claude Code does not support project commands.
func (a *Adapter) ProjectCommandsDir(_ string) string {
	return ""
}

// ConfigDir returns the root config directory for Claude Code.
func ConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".claude")
}

// DelegationStrategy returns DelegationNativeAgents — Claude Code supports .claude/agents/*.md sub-agents.
func (a *Adapter) DelegationStrategy() domain.DelegationStrategy {
	return domain.DelegationNativeAgents
}

// SupportsSubAgents returns true — Claude Code supports named sub-agent files.
func (a *Adapter) SupportsSubAgents() bool {
	return true
}

// SubAgentsDir returns ~/.claude/agents — the user-scope sub-agents directory.
func (a *Adapter) SubAgentsDir(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "agents")
}

// SupportsWorkflows returns false — Claude Code does not support workflow files.
func (a *Adapter) SupportsWorkflows() bool {
	return false
}

// WorkflowsDir returns empty string — Claude Code does not support workflows.
func (a *Adapter) WorkflowsDir(_ string) string {
	return ""
}

// MCPRootKey returns "mcpServers" — Claude Code uses the standard mcpServers key.
func (a *Adapter) MCPRootKey() string { return "mcpServers" }

// MCPURLKey returns "url" — Claude Code uses the standard URL key.
func (a *Adapter) MCPURLKey() string { return "url" }

// MCPConfigPath returns <projectDir>/.mcp.json.
func (a *Adapter) MCPConfigPath(projectDir string) string {
	return filepath.Join(projectDir, ".mcp.json")
}

// RulesFrontmatter returns empty string — Claude Code uses marker-based injection.
func (a *Adapter) RulesFrontmatter() string { return "" }
