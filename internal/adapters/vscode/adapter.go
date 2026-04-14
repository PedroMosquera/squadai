package vscode

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
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

// SystemPromptFile returns the path to the Copilot instructions file.
// macOS: ~/Library/Application Support/Code/User/.instructions.md
// Linux: ~/.config/Code/User/.instructions.md
func (a *Adapter) SystemPromptFile(homeDir string) string {
	return filepath.Join(ConfigDir(homeDir), ".instructions.md")
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
		domain.ComponentMCP, domain.ComponentSkills:
		return true
	default:
		return false
	}
}

// ProjectConfigFile returns <projectDir>/.vscode/settings.json.
func (a *Adapter) ProjectConfigFile(projectDir string) string {
	return filepath.Join(projectDir, ".vscode", "settings.json")
}

// ProjectRulesFile returns <projectDir>/.instructions.md.
func (a *Adapter) ProjectRulesFile(projectDir string) string {
	return filepath.Join(projectDir, ".instructions.md")
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

// ConfigDir returns the root config directory for VS Code Copilot.
// On macOS it is ~/Library/Application Support/Code/User.
// On Linux it is ~/.config/Code/User.
func ConfigDir(homeDir string) string {
	if runtime.GOOS == "linux" {
		return filepath.Join(homeDir, ".config", "Code", "User")
	}
	return filepath.Join(homeDir, "Library", "Application Support", "Code", "User")
}
