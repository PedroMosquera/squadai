package plugins

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
)

// pluginSkillContent maps plugin names to their skill-file content.
// Used when a plugin's InstallMethod is "skill_files".
var pluginSkillContent = map[string]string{
	"superpowers": "# Superpowers\n\nEnhanced coding capabilities with advanced code generation, refactoring, and analysis tools.\n",
}

// Installer implements domain.ComponentInstaller for third-party plugin installation.
// It supports three install methods:
//   - claude_plugin: declares the plugin, and optionally its marketplace, in
//     Claude Code's project settings; Claude Code installs it. Other agents
//     have no way to load a Claude plugin, so nothing is written for them.
//   - package: adds an npm or git package to the array Pi or OpenCode reads
//     plugin packages from; the agent installs it.
//   - skill_files: writes a skill markdown file to the adapter's skills directory
type Installer struct {
	plugins map[string]domain.PluginDef
	config  *domain.MergedConfig

	// projectDir is captured during Plan so Apply can record ownership in the
	// managed sidecar.
	projectDir string
}

// New returns a plugin installer. cfg may be nil for backward compatibility.
func New(plugins map[string]domain.PluginDef, cfg *domain.MergedConfig) *Installer {
	return &Installer{
		plugins: plugins,
		config:  cfg,
	}
}

// ID returns the component identifier.
func (i *Installer) ID() domain.ComponentID {
	return domain.ComponentPlugins
}

// Plan determines what plugin actions are needed for the given adapter.
func (i *Installer) Plan(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	if !adapter.SupportsComponent(domain.ComponentPlugins) {
		return nil, nil
	}
	i.projectDir = projectDir

	var actions []domain.PlannedAction
	if adapter.ID() == domain.AgentClaudeCode {
		a, err := i.planClaudeSettings(adapter, projectDir)
		if err != nil {
			return nil, err
		}
		actions = append(actions, a...)
	}
	pkgActions, err := i.planPackages(adapter, projectDir)
	if err != nil {
		return nil, err
	}
	actions = append(actions, pkgActions...)

	for _, name := range sortedPluginNames(i.plugins) {
		plugin := i.plugins[name]
		if plugin.InstallMethod != "skill_files" || !i.active(plugin, adapter.ID()) {
			continue
		}
		a, err := i.planSkillFile(adapter, name, plugin, projectDir)
		if err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}

	return actions, nil
}

// active reports whether a declaration applies to the agent: enabled, not
// excluded by the methodology, and listed in supported_agents.
func (i *Installer) active(plugin domain.PluginDef, agent domain.AgentID) bool {
	if !plugin.Enabled {
		return false
	}
	if i.config != nil && plugin.ExcludesMethodology != "" &&
		plugin.ExcludesMethodology == string(i.config.Methodology) {
		return false
	}
	return isAgentSupported(string(agent), plugin.SupportedAgents)
}

// planSkillFile plans writing a skill file for a plugin.
func (i *Installer) planSkillFile(adapter domain.Adapter, name string, _ domain.PluginDef, projectDir string) (domain.PlannedAction, error) {
	skillsDir := adapter.ProjectSkillsDir(projectDir)
	if skillsDir == "" {
		// Fall back to global skills dir if no project skills.
		return domain.PlannedAction{
			ID:          fmt.Sprintf("%s-plugin-%s", adapter.ID(), name),
			Agent:       adapter.ID(),
			Component:   domain.ComponentPlugins,
			Action:      domain.ActionSkip,
			TargetPath:  "",
			Description: fmt.Sprintf("plugin:skill:%s", name),
		}, nil
	}

	targetPath := filepath.Join(skillsDir, name, "SKILL.md")
	actionID := fmt.Sprintf("%s-plugin-%s", adapter.ID(), name)
	description := fmt.Sprintf("plugin:skill:%s", name)

	content := pluginSkillContentFor(name)

	existing, err := fileutil.ReadFileOrEmpty(targetPath)
	if err != nil {
		return domain.PlannedAction{}, fmt.Errorf("read skill for plugin %s: %w", name, err)
	}

	if string(existing) == content {
		return domain.PlannedAction{
			ID:          actionID,
			Agent:       adapter.ID(),
			Component:   domain.ComponentPlugins,
			Action:      domain.ActionSkip,
			TargetPath:  targetPath,
			Description: description,
		}, nil
	}

	action := domain.ActionCreate
	if len(existing) > 0 {
		action = domain.ActionUpdate
	}

	return domain.PlannedAction{
		ID:          actionID,
		Agent:       adapter.ID(),
		Component:   domain.ComponentPlugins,
		Action:      action,
		TargetPath:  targetPath,
		Description: description,
	}, nil
}

// Apply executes a single planned action.
func (i *Installer) Apply(action domain.PlannedAction) error {
	if action.Action == domain.ActionSkip {
		return nil
	}

	switch {
	case action.Description == claudeSettingsDescription:
		return i.applyClaudeSettings(action)
	case action.Description == packagesDescription:
		return i.applyPackages(action)
	case strings.HasPrefix(action.Description, "plugin:skill:"):
		return i.applySkillFile(action)
	default:
		return fmt.Errorf("unknown plugin action: %s", action.Description)
	}
}

// applySkillFile writes a skill file for a plugin.
func (i *Installer) applySkillFile(action domain.PlannedAction) error {
	name := strings.TrimPrefix(action.Description, "plugin:skill:")
	content := pluginSkillContentFor(name)

	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create skill dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write plugin skill: %w", err)
	}

	return nil
}

// Verify checks post-apply state for the plugins component.
func (i *Installer) Verify(adapter domain.Adapter, homeDir, projectDir string) ([]domain.VerifyResult, error) {
	if !adapter.SupportsComponent(domain.ComponentPlugins) {
		return nil, nil
	}

	var results []domain.VerifyResult
	if adapter.ID() == domain.AgentClaudeCode {
		r, err := i.verifyClaudeSettings(adapter, projectDir)
		if err != nil {
			return nil, err
		}
		results = append(results, r...)
	}
	results = append(results, i.verifyPackages(adapter, projectDir)...)

	for _, name := range sortedPluginNames(i.plugins) {
		plugin := i.plugins[name]
		if plugin.InstallMethod != "skill_files" || !i.active(plugin, adapter.ID()) {
			continue
		}
		results = append(results, i.verifySkillFile(adapter, name, projectDir))
	}

	return results, nil
}

// verifySkillFile checks that the skill file for a plugin exists and matches.
func (i *Installer) verifySkillFile(adapter domain.Adapter, name string, projectDir string) domain.VerifyResult {
	checkName := fmt.Sprintf("plugin-%s-skill-exists", name)

	skillsDir := adapter.ProjectSkillsDir(projectDir)
	if skillsDir == "" {
		return domain.VerifyResult{
			Check:     checkName,
			Passed:    false,
			Severity:  domain.SeverityError,
			Component: "plugins",
			Message:   "adapter has no project skills directory",
		}
	}

	targetPath := filepath.Join(skillsDir, name, "SKILL.md")
	expected := pluginSkillContentFor(name)

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return domain.VerifyResult{
			Check:     checkName,
			Passed:    false,
			Severity:  domain.SeverityError,
			Component: "plugins",
			Message:   fmt.Sprintf("skill file not found: %s", targetPath),
		}
	}

	if string(data) == expected {
		return domain.VerifyResult{
			Check:     checkName,
			Passed:    true,
			Severity:  domain.SeverityInfo,
			Component: "plugins",
		}
	}

	return domain.VerifyResult{
		Check:     checkName,
		Passed:    false,
		Severity:  domain.SeverityError,
		Component: "plugins",
		Message:   fmt.Sprintf("plugin skill %s content does not match expected", name),
	}
}

// RenderContent returns the content that Apply would write for the given action,
// without performing the write. Used by the diff renderer.
func (i *Installer) RenderContent(action domain.PlannedAction) ([]byte, error) {
	switch {
	case action.Description == claudeSettingsDescription:
		return i.renderClaudeSettings(action)
	case action.Description == packagesDescription:
		return i.renderPackages(action)
	case strings.HasPrefix(action.Description, "plugin:skill:"):
		name := strings.TrimPrefix(action.Description, "plugin:skill:")
		return []byte(pluginSkillContentFor(name)), nil
	default:
		return nil, fmt.Errorf("unknown plugin action: %s", action.Description)
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// isAgentSupported checks if agentID is in the supported agents list.
func isAgentSupported(agentID string, supported []string) bool {
	for _, s := range supported {
		if s == agentID {
			return true
		}
	}
	return false
}

// pluginSkillContentFor returns the skill file content for a plugin.
// Falls back to a generic description if no specific content is available.
func pluginSkillContentFor(name string) string {
	if content, ok := pluginSkillContent[name]; ok {
		return content
	}
	return fmt.Sprintf("# %s\n\nPlugin-provided skill.\n", name)
}

// sortedPluginNames returns plugin names in sorted order for deterministic output.
func sortedPluginNames(m map[string]domain.PluginDef) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
