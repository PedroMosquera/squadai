package windsurf

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/PedroMosquera/squadai/internal/domain"
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
//
// This is Windsurf's user-scoped "always-on" rules file (6,000 character
// cap). As of 2026 it lives directly under the Windsurf config dir; the
// sibling memories/ directory is reserved for Cascade-generated context
// and must not be written to by external tooling. Verified against
// docs.windsurf.com/windsurf/cascade/memories.
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

// ProjectRulesFile returns <projectDir>/.windsurf/rules/squadai.md.
// Uses the structured rules format with YAML frontmatter (trigger: always_on).
func (a *Adapter) ProjectRulesFile(projectDir string) string {
	return filepath.Join(projectDir, ".windsurf", "rules", "squadai.md")
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

// MCPRootKey returns "mcpServers" — Windsurf uses the standard mcpServers key.
func (a *Adapter) MCPRootKey() string { return "mcpServers" }

// MCPURLKey returns "serverUrl" — Windsurf uses a non-standard URL key.
func (a *Adapter) MCPURLKey() string { return "serverUrl" }

// MCPConfigPath returns <projectDir>/.windsurf/mcp_config.json.
func (a *Adapter) MCPConfigPath(projectDir string) string {
	return filepath.Join(projectDir, ".windsurf", "mcp_config.json")
}

// MCPCommandStyle returns "split" — Windsurf uses command + args.
func (a *Adapter) MCPCommandStyle() string { return "split" }

// MCPEnvKey returns "env" — Windsurf uses the standard env key.
func (a *Adapter) MCPEnvKey() string { return "env" }

// MCPTypeField always returns empty string — Windsurf infers stdio vs remote from URL presence.
func (a *Adapter) MCPTypeField(_ domain.MCPServerDef) string { return "" }

// RulesFrontmatter returns YAML frontmatter for Windsurf's structured rules.
func (a *Adapter) RulesFrontmatter() string {
	return "---\ntrigger: always_on\n---\n\n"
}

// RulesFileSizeCap returns 6000 — Windsurf's global_rules.md has a hard 6,000-character cap.
func (a *Adapter) RulesFileSizeCap() int { return 6000 }

// ConfigDir returns the root config directory for Windsurf.
// On Windows it is %APPDATA%\Windsurf\User (falling back to homeDir\AppData\Roaming\Windsurf\User).
// On all other platforms it is ~/.codeium/windsurf.
func ConfigDir(homeDir string) string {
	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, "Windsurf", "User")
		}
		return filepath.Join(homeDir, "AppData", "Roaming", "Windsurf", "User")
	}
	return filepath.Join(homeDir, ".codeium", "windsurf")
}
