package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// OverrideSpec describes path overrides for a built-in adapter.
// Fields left empty fall back to the built-in adapter's defaults.
type OverrideSpec struct {
	ConfigDir     string   `json:"config_dir,omitempty"`     // e.g. "~/.pi/agent"
	AgentsSubdir  string   `json:"agents_subdir,omitempty"`  // e.g. "agents"
	PromptsSubdir string   `json:"prompts_subdir,omitempty"` // e.g. "prompts"
	SkillsSubdir  string   `json:"skills_subdir,omitempty"`  // e.g. "skills"
	SettingsPath  string   `json:"settings_path,omitempty"`  // e.g. "settings.json"
	Delegation    string   `json:"delegation,omitempty"`     // "native", "prompt", "solo"
	Supports      []string `json:"supports,omitempty"`       // component IDs to add
	Unsupported   []string `json:"unsupported,omitempty"`    // component IDs to remove
}

// OverrideAdapter wraps a built-in adapter and overrides selected path methods.
type OverrideAdapter struct {
	base    domain.Adapter
	spec    OverrideSpec
	homeDir string // cached for path expansion
}

// expandPath replaces a leading ~ with homeDir.
func expandPath(path, homeDir string) string {
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, path[2:])
	}
	if path == "~" {
		return homeDir
	}
	return path
}

// expandedConfigDir returns the override config dir expanded against homeDir.
func (o *OverrideAdapter) expandedConfigDir(homeDir string) string {
	return expandPath(o.spec.ConfigDir, homeDir)
}

// ─── Overridable path methods ───────────────────────────────────────────────

// GlobalConfigDir returns the override config dir when set, else delegates.
func (o *OverrideAdapter) GlobalConfigDir(homeDir string) string {
	if o.spec.ConfigDir != "" {
		return o.expandedConfigDir(homeDir)
	}
	return o.base.GlobalConfigDir(homeDir)
}

// SystemPromptFile returns <configDir>/AGENTS.md when ConfigDir is set, else delegates.
func (o *OverrideAdapter) SystemPromptFile(homeDir string) string {
	if o.spec.ConfigDir != "" {
		return filepath.Join(o.expandedConfigDir(homeDir), "AGENTS.md")
	}
	return o.base.SystemPromptFile(homeDir)
}

// SkillsDir returns <configDir>/<skillsSubdir> when ConfigDir is set, else delegates.
func (o *OverrideAdapter) SkillsDir(homeDir string) string {
	if o.spec.ConfigDir != "" {
		sub := o.spec.SkillsSubdir
		if sub == "" {
			sub = "skills"
		}
		return filepath.Join(o.expandedConfigDir(homeDir), sub)
	}
	return o.base.SkillsDir(homeDir)
}

// agentsSubdirOrDefault returns the configured agents subdir or "agents".
func (o *OverrideAdapter) agentsSubdirOrDefault() string {
	sub := o.spec.AgentsSubdir
	if sub == "" {
		sub = "agents"
	}
	return sub
}

// AgentsDir returns <configDir>/<agentsSubdir> when ConfigDir is set; otherwise
// delegates to the base adapter when it exposes a global AgentsDir method.
func (o *OverrideAdapter) AgentsDir(homeDir string) string {
	if o.spec.ConfigDir != "" {
		return filepath.Join(o.expandedConfigDir(homeDir), o.agentsSubdirOrDefault())
	}
	if d, ok := o.base.(interface{ AgentsDir(string) string }); ok {
		return d.AgentsDir(homeDir)
	}
	return ""
}

// SubAgentsDir returns <configDir>/<agentsSubdir> when ConfigDir is set, else delegates.
func (o *OverrideAdapter) SubAgentsDir(homeDir string) string {
	if o.spec.ConfigDir != "" {
		return filepath.Join(o.expandedConfigDir(homeDir), o.agentsSubdirOrDefault())
	}
	return o.base.SubAgentsDir(homeDir)
}

// SettingsPath returns <configDir>/<settingsPath> when ConfigDir is set, else delegates.
func (o *OverrideAdapter) SettingsPath(homeDir string) string {
	if o.spec.ConfigDir != "" {
		p := o.spec.SettingsPath
		if p == "" {
			p = "settings.json"
		}
		return filepath.Join(o.expandedConfigDir(homeDir), p)
	}
	return o.base.SettingsPath(homeDir)
}

// DelegationStrategy returns the override strategy when set, else delegates.
func (o *OverrideAdapter) DelegationStrategy() domain.DelegationStrategy {
	if o.spec.Delegation != "" {
		return domain.DelegationStrategy(o.spec.Delegation)
	}
	return o.base.DelegationStrategy()
}

// SupportsComponent starts from the base adapter's support map, adds any
// explicitly supported components, then removes any unsupported ones.
func (o *OverrideAdapter) SupportsComponent(c domain.ComponentID) bool {
	result := o.base.SupportsComponent(c)
	for _, s := range o.spec.Supports {
		if domain.ComponentID(s) == c {
			result = true
		}
	}
	for _, u := range o.spec.Unsupported {
		if domain.ComponentID(u) == c {
			result = false
		}
	}
	return result
}

// ─── Delegating methods ─────────────────────────────────────────────────────

// ID returns the base adapter's identifier.
func (o *OverrideAdapter) ID() domain.AgentID { return o.base.ID() }

// Lane returns the base adapter's lane.
func (o *OverrideAdapter) Lane() domain.AdapterLane { return o.base.Lane() }

// Detect delegates to the base adapter.
func (o *OverrideAdapter) Detect(ctx context.Context, homeDir string) (installed bool, configFound bool, err error) {
	return o.base.Detect(ctx, homeDir)
}

// ProjectConfigFile delegates to the base adapter.
func (o *OverrideAdapter) ProjectConfigFile(projectDir string) string {
	return o.base.ProjectConfigFile(projectDir)
}

// ProjectRulesFile delegates to the base adapter.
func (o *OverrideAdapter) ProjectRulesFile(projectDir string) string {
	return o.base.ProjectRulesFile(projectDir)
}

// ProjectAgentsDir delegates to the base adapter.
func (o *OverrideAdapter) ProjectAgentsDir(projectDir string) string {
	return o.base.ProjectAgentsDir(projectDir)
}

// ProjectSkillsDir delegates to the base adapter.
func (o *OverrideAdapter) ProjectSkillsDir(projectDir string) string {
	return o.base.ProjectSkillsDir(projectDir)
}

// ProjectCommandsDir delegates to the base adapter.
func (o *OverrideAdapter) ProjectCommandsDir(projectDir string) string {
	return o.base.ProjectCommandsDir(projectDir)
}

// CommandsDir delegates to the base adapter when it exposes a global
// CommandsDir method. Not all adapters implement this method.
func (o *OverrideAdapter) CommandsDir(homeDir string) string {
	if d, ok := o.base.(interface{ CommandsDir(string) string }); ok {
		return d.CommandsDir(homeDir)
	}
	return ""
}

// SupportsSubAgents delegates to the base adapter.
func (o *OverrideAdapter) SupportsSubAgents() bool { return o.base.SupportsSubAgents() }

// SupportsWorkflows delegates to the base adapter.
func (o *OverrideAdapter) SupportsWorkflows() bool { return o.base.SupportsWorkflows() }

// WorkflowsDir delegates to the base adapter.
func (o *OverrideAdapter) WorkflowsDir(projectDir string) string {
	return o.base.WorkflowsDir(projectDir)
}

// MCPRootKey delegates to the base adapter.
func (o *OverrideAdapter) MCPRootKey() string { return o.base.MCPRootKey() }

// MCPURLKey delegates to the base adapter.
func (o *OverrideAdapter) MCPURLKey() string { return o.base.MCPURLKey() }

// MCPConfigPath delegates to the base adapter.
func (o *OverrideAdapter) MCPConfigPath(projectDir string) string {
	return o.base.MCPConfigPath(projectDir)
}

// MCPCommandStyle delegates to the base adapter.
func (o *OverrideAdapter) MCPCommandStyle() string { return o.base.MCPCommandStyle() }

// MCPEnvKey delegates to the base adapter.
func (o *OverrideAdapter) MCPEnvKey() string { return o.base.MCPEnvKey() }

// MCPTypeField delegates to the base adapter.
func (o *OverrideAdapter) MCPTypeField(def domain.MCPServerDef) string {
	return o.base.MCPTypeField(def)
}

// RulesFrontmatter delegates to the base adapter.
func (o *OverrideAdapter) RulesFrontmatter() string { return o.base.RulesFrontmatter() }

// RulesFileSizeCap delegates to the base adapter.
func (o *OverrideAdapter) RulesFileSizeCap() int { return o.base.RulesFileSizeCap() }

// ─── Loading & applying overrides ───────────────────────────────────────────

// LoadOverride reads an override spec from .squadai/adapters/<id>.json.
// Returns nil, nil if the file does not exist (no override).
func LoadOverride(projectDir string, agentID domain.AgentID) (*OverrideSpec, error) {
	path := filepath.Join(projectDir, ".squadai", "adapters", string(agentID)+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read adapter override %s: %w", agentID, err)
	}
	var spec OverrideSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parse adapter override %s: %w", agentID, err)
	}
	return &spec, nil
}

// ApplyOverride wraps base with an OverrideAdapter if an override file exists
// for base.ID() in projectDir. If no override file exists, base is returned
// unchanged.
func ApplyOverride(base domain.Adapter, projectDir string) (domain.Adapter, error) {
	spec, err := LoadOverride(projectDir, base.ID())
	if err != nil {
		return nil, err
	}
	if spec == nil {
		return base, nil
	}
	return &OverrideAdapter{base: base, spec: *spec}, nil
}

// ApplyOverrides applies overrides to all adapters in the slice.
// Adapters without override files are returned unchanged.
func ApplyOverrides(adapters []domain.Adapter, projectDir string) ([]domain.Adapter, error) {
	result := make([]domain.Adapter, 0, len(adapters))
	for _, a := range adapters {
		overridden, err := ApplyOverride(a, projectDir)
		if err != nil {
			return nil, fmt.Errorf("adapter %s: %w", a.ID(), err)
		}
		result = append(result, overridden)
	}
	return result, nil
}

// EffectivePaths returns the effective paths for an adapter after applying
// any override. Useful for 'squadai explain adapter <id>' and doctor checks.
func EffectivePaths(adapter domain.Adapter, homeDir, projectDir string) map[string]string {
	paths := map[string]string{
		"config_dir":     adapter.GlobalConfigDir(homeDir),
		"system_prompt":  adapter.SystemPromptFile(homeDir),
		"skills_dir":     adapter.SkillsDir(homeDir),
		"settings_path":  adapter.SettingsPath(homeDir),
		"sub_agents_dir": adapter.SubAgentsDir(homeDir),
		"delegation":     string(adapter.DelegationStrategy()),
	}
	if d, ok := adapter.(interface{ AgentsDir(string) string }); ok {
		paths["agents_dir"] = d.AgentsDir(homeDir)
	} else {
		paths["agents_dir"] = ""
	}
	return paths
}

// Compile-time assertion that OverrideAdapter satisfies domain.Adapter.
var _ domain.Adapter = (*OverrideAdapter)(nil)
