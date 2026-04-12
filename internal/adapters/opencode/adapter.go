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
// V1: memory is the only supported component.
func (a *Adapter) SupportsComponent(c domain.ComponentID) bool {
	switch c {
	case domain.ComponentMemory:
		return true
	default:
		return false
	}
}

// ConfigDir returns the root config directory for OpenCode.
func ConfigDir(homeDir string) string {
	return filepath.Join(homeDir, ".config", "opencode")
}
