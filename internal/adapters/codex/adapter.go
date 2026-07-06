package codex

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// Adapter implements domain.Adapter for the OpenAI Codex CLI.
// Codex is a personal-lane, solo-delegation agent. It reads project
// instructions from AGENTS.md at the repo root (shared with OpenCode and Pi
// via adapter-scoped markers), global instructions from ~/.codex/AGENTS.md,
// and MCP servers from [mcp_servers.<name>] tables in ~/.codex/config.toml.
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
	return domain.AgentCodex
}

// Lane returns the adapter lane. Codex is a personal-optional adapter.
func (a *Adapter) Lane() domain.AdapterLane {
	return domain.LanePersonal
}

// Detect checks whether the codex binary is on PATH and whether
// the config directory (~/.codex) exists.
func (a *Adapter) Detect(_ context.Context, homeDir string) (installed bool, configFound bool, err error) {
	_, lookErr := a.lookPath("codex")
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

// GlobalConfigDir returns ~/.codex.
func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return ConfigDir(homeDir)
}

// SystemPromptFile returns ~/.codex/AGENTS.md — Codex's global instructions file.
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "AGENTS.md")
}

// SkillsDir returns empty string — Codex has no skills directory.
func (a *Adapter) SkillsDir(_ string) string {
	return ""
}

// SettingsPath returns ~/.codex/config.toml — Codex's global TOML config.
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "config.toml")
}

// SupportsComponent reports whether Codex supports a given component.
// Codex has no sub-agent files, skills, commands, or permission policy;
// team orchestration is injected into the rules file via the solo
// delegation strategy, which bypasses the ComponentAgents guard.
func (a *Adapter) SupportsComponent(c domain.ComponentID) bool {
	switch c {
	case domain.ComponentMemory, domain.ComponentRules, domain.ComponentMCP,
		domain.ComponentBrand, domain.ComponentEfficiency:
		return true
	default:
		return false
	}
}

// ProjectConfigFile returns empty string — Codex has no project-level JSON config.
func (a *Adapter) ProjectConfigFile(_ string) string {
	return ""
}

// ProjectRulesFile returns <projectDir>/AGENTS.md — Codex reads project
// instructions from AGENTS.md at the repo root.
func (a *Adapter) ProjectRulesFile(projectDir string) string {
	return filepath.Join(projectDir, "AGENTS.md")
}

// ProjectAgentsDir returns empty string — Codex is a solo agent with no sub-agents.
func (a *Adapter) ProjectAgentsDir(_ string) string {
	return ""
}

// ProjectSkillsDir returns empty string — Codex does not support skills.
func (a *Adapter) ProjectSkillsDir(_ string) string {
	return ""
}

// ProjectCommandsDir returns empty string — Codex does not support commands.
func (a *Adapter) ProjectCommandsDir(_ string) string {
	return ""
}

// DelegationStrategy returns DelegationSoloAgent — Codex runs all phases inline.
func (a *Adapter) DelegationStrategy() domain.DelegationStrategy {
	return domain.DelegationSoloAgent
}

// SupportsSubAgents returns false — Codex is a solo agent.
func (a *Adapter) SupportsSubAgents() bool {
	return false
}

// SubAgentsDir returns empty string — Codex does not support sub-agents.
func (a *Adapter) SubAgentsDir(_ string) string {
	return ""
}

// SupportsWorkflows returns false — Codex does not support workflow files.
func (a *Adapter) SupportsWorkflows() bool {
	return false
}

// WorkflowsDir returns empty string — Codex does not support workflows.
func (a *Adapter) WorkflowsDir(_ string) string {
	return ""
}

// MCPRootKey returns "mcp_servers" — Codex nests MCP servers under
// [mcp_servers.<name>] tables in config.toml.
func (a *Adapter) MCPRootKey() string { return "mcp_servers" }

// MCPURLKey returns "url" — Codex uses the standard URL key for
// streamable HTTP MCP servers.
func (a *Adapter) MCPURLKey() string { return "url" }

// MCPConfigPath returns empty string — Codex has no project-level MCP config
// file. MCP servers are written to the global TOML config; see
// MCPTOMLConfigPath.
func (a *Adapter) MCPConfigPath(_ string) string { return "" }

// MCPTOMLConfigPath returns ~/.codex/config.toml. This is a Codex-specific
// method (not part of domain.Adapter); the MCP installer type-asserts for it
// to select the marker-managed TOML strategy.
func (a *Adapter) MCPTOMLConfigPath(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "config.toml")
}

// MCPCommandStyle returns "split" — Codex uses command + args keys.
func (a *Adapter) MCPCommandStyle() string { return "split" }

// MCPEnvKey returns "env" — Codex uses the standard env key (inline table).
func (a *Adapter) MCPEnvKey() string { return "env" }

// MCPTypeField always returns empty string — Codex infers stdio vs remote
// from the presence of command vs url keys.
func (a *Adapter) MCPTypeField(_ domain.MCPServerDef) string { return "" }

// RulesFrontmatter returns empty string — Codex uses marker-based injection.
func (a *Adapter) RulesFrontmatter() string { return "" }

// RulesFileSizeCap returns 0 — Codex has no known rules file size limit.
func (a *Adapter) RulesFileSizeCap() int { return 0 }

// ConfigDir returns the root config directory for Codex.
func ConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".codex")
}
