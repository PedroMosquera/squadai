package vscode

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/PedroMosquera/squadai/internal/adapters/paths"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// Adapter implements domain.Adapter for VS Code Copilot.
// VS Code Copilot is a personal-lane solo agent — no sub-agent delegation.
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
	return domain.AgentVSCodeCopilot
}

// Lane returns the adapter lane. VS Code Copilot is personal-optional.
func (a *Adapter) Lane() domain.AdapterLane {
	return domain.LanePersonal
}

// Detect checks whether the code binary is on PATH and whether
// the config directory exists for the current OS.
// macOS: ~/Library/Application Support/Code/User
// Linux: ~/.config/Code/User
func (a *Adapter) Detect(_ context.Context, homeDir string) (installed bool, configFound bool, err error) {
	_, lookErr := a.lookPath("code")
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

// GlobalConfigDir returns the VS Code user config directory for the current OS.
// macOS: ~/Library/Application Support/Code/User
// Linux: ~/.config/Code/User
func (a *Adapter) GlobalConfigDir(homeDir string) string {
	return ConfigDir(homeDir)
}

// SystemPromptFile returns ~/.copilot/copilot-instructions.md, the personal
// always-on instructions file VS Code reads in Copilot Agent Host sessions
// without any extra setting.
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(homeDir, ".copilot", "copilot-instructions.md")
}

// SkillsDir returns ~/.copilot/skills (same on all platforms).
func (a *Adapter) SkillsDir(homeDir string) string {
	return filepath.Join(homeDir, ".copilot", "skills")
}

// SettingsPath returns the VS Code settings.json path for the current OS.
// macOS: ~/Library/Application Support/Code/User/settings.json
// Linux: ~/.config/Code/User/settings.json
func (a *Adapter) SettingsPath(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), "settings.json")
}

// SupportsComponent reports whether VS Code Copilot supports a given component.
func (a *Adapter) SupportsComponent(c domain.ComponentID) bool {
	switch c {
	case domain.ComponentMemory, domain.ComponentRules, domain.ComponentSettings,
		domain.ComponentMCP, domain.ComponentSkills, domain.ComponentPlugins,
		domain.ComponentPermissions, domain.ComponentBrand, domain.ComponentEfficiency:
		return true
	default:
		return false
	}
}

// ProjectConfigFile returns <projectDir>/.vscode/settings.json.
func (a *Adapter) ProjectConfigFile(projectDir string) string {
	return filepath.Join(projectDir, ".vscode", "settings.json")
}

// ProjectRulesFile returns <projectDir>/.github/copilot-instructions.md. The
// copilot instructions template writes its own marker section into the same
// file; the two coexist because their section IDs differ.
func (a *Adapter) ProjectRulesFile(projectDir string) string {
	return filepath.Join(projectDir, ".github", "copilot-instructions.md")
}

// ProjectAgentsDir returns empty string — VS Code Copilot does not support project agents.
func (a *Adapter) ProjectAgentsDir(_ string) string {
	return ""
}

// ProjectSkillsDir returns <projectDir>/.copilot/skills.
func (a *Adapter) ProjectSkillsDir(projectDir string) string {
	return filepath.Join(projectDir, ".copilot", "skills")
}

// ProjectCommandsDir returns empty string — VS Code Copilot does not support project commands.
func (a *Adapter) ProjectCommandsDir(_ string) string {
	return ""
}

// DelegationStrategy returns DelegationSoloAgent — VS Code Copilot runs all phases inline.
func (a *Adapter) DelegationStrategy() domain.DelegationStrategy {
	return domain.DelegationSoloAgent
}

// SupportsSubAgents returns false — VS Code Copilot does not create named sub-agent files.
func (a *Adapter) SupportsSubAgents() bool {
	return false
}

// SubAgentsDir returns empty string — VS Code Copilot does not support sub-agent files.
func (a *Adapter) SubAgentsDir(_ string) string {
	return ""
}

// SupportsWorkflows returns false — VS Code Copilot does not support workflow files.
func (a *Adapter) SupportsWorkflows() bool {
	return false
}

// WorkflowsDir returns empty string — VS Code Copilot does not support workflows.
func (a *Adapter) WorkflowsDir(_ string) string {
	return ""
}

// MCPRootKey returns "mcpServers", the key of the portable workspace format
// that VS Code and the Agent Host both read from <project>/.mcp.json.
func (a *Adapter) MCPRootKey() string { return "mcpServers" }

// MCPURLKey returns "url" — VS Code Copilot uses the standard URL key.
func (a *Adapter) MCPURLKey() string { return "url" }

// MCPConfigPath returns <projectDir>/.mcp.json, the same file and entry shape
// the Claude Code adapter writes. Both adapters must keep serializing entries
// identically or a shared apply will flap between them.
func (a *Adapter) MCPConfigPath(projectDir string) string {
	return filepath.Join(projectDir, ".mcp.json")
}

// LegacyMCPConfig returns the location squadai wrote before VS Code 1.140. The
// Agent Host does not read it directly, so the MCP installer moves
// squadai-owned servers out of it.
func (a *Adapter) LegacyMCPConfig(projectDir string) (path, rootKey string) {
	return filepath.Join(projectDir, ".vscode", "mcp.json"), "servers"
}

// LegacyInstructionsFiles returns where squadai wrote instructions before it
// targeted files VS Code reads. Neither is a documented instructions location,
// so squadai's marker blocks there are dead weight.
func (a *Adapter) LegacyInstructionsFiles(homeDir, projectDir string) []string {
	return []string{
		filepath.Join(projectDir, ".instructions.md"),
		filepath.Join(ConfigDir(homeDir), ".instructions.md"),
	}
}

// MCPCommandStyle returns "split" — VS Code Copilot uses command + args.
func (a *Adapter) MCPCommandStyle() string { return "split" }

// MCPEnvKey returns "env" — VS Code Copilot uses the standard env key.
func (a *Adapter) MCPEnvKey() string { return "env" }

// MCPTypeField returns "http" for remote servers and empty string for stdio.
func (a *Adapter) MCPTypeField(def domain.MCPServerDef) string {
	if def.URL != "" {
		return "http"
	}
	return ""
}

// RulesFrontmatter returns empty string — VS Code Copilot uses YAML frontmatter in .instructions.md but not for rules.
func (a *Adapter) RulesFrontmatter() string { return "" }

// RulesFileSizeCap returns 0 — VS Code Copilot has no known rules file size limit.
func (a *Adapter) RulesFileSizeCap() int { return 0 }

// ConfigDir returns the root config directory for VS Code Copilot.
// VS Code follows the standard Electron user-data convention on every OS,
// so this delegates fully to paths.UserConfigDir.
func ConfigDir(homeDir string) string {
	return paths.UserConfigDir(homeDir, runtime.GOOS, "Code")
}
