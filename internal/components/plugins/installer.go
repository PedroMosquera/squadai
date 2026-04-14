package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/fileutil"
)

// pluginSkillContent maps plugin names to their skill-file content.
// Used when a plugin's InstallMethod is "skill_files".
var pluginSkillContent = map[string]string{
	"superpowers": "# Superpowers\n\nEnhanced coding capabilities with advanced code generation, refactoring, and analysis tools.\n",
}

// Installer implements domain.ComponentInstaller for third-party plugin installation.
// It supports two install methods:
//   - claude_plugin: writes to the adapter's settings.json under "enabledPlugins"
//   - skill_files: writes a skill markdown file to the adapter's skills directory
type Installer struct {
	plugins map[string]domain.PluginDef
	config  *domain.MergedConfig
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

	if len(i.plugins) == 0 {
		return nil, nil
	}

	var actions []domain.PlannedAction
	agentID := string(adapter.ID())

	for _, name := range sortedPluginNames(i.plugins) {
		plugin := i.plugins[name]

		// Skip disabled plugins.
		if !plugin.Enabled {
			continue
		}

		// Skip if methodology is excluded.
		if i.config != nil && plugin.ExcludesMethodology != "" &&
			plugin.ExcludesMethodology == string(i.config.Methodology) {
			continue
		}

		// Skip if adapter is not in supported agents list.
		if !isAgentSupported(agentID, plugin.SupportedAgents) {
			continue
		}

		switch effectiveInstallMethod(plugin, adapter) {
		case "claude_plugin":
			a, err := i.planClaudePlugin(adapter, name, plugin, homeDir)
			if err != nil {
				return nil, err
			}
			actions = append(actions, a)

		case "skill_files":
			a, err := i.planSkillFile(adapter, name, plugin, projectDir)
			if err != nil {
				return nil, err
			}
			actions = append(actions, a)
		}
	}

	return actions, nil
}

// planClaudePlugin plans an update to the adapter's settings.json to enable the plugin.
func (i *Installer) planClaudePlugin(adapter domain.Adapter, name string, plugin domain.PluginDef, homeDir string) (domain.PlannedAction, error) {
	targetPath := adapter.SettingsPath(homeDir)
	actionID := fmt.Sprintf("%s-plugin-%s", adapter.ID(), name)
	description := fmt.Sprintf("plugin:claude:%s", name)

	// Check if already enabled.
	existing, err := readJSONFile(targetPath)
	if err != nil {
		return domain.PlannedAction{}, fmt.Errorf("read settings for plugin %s: %w", name, err)
	}

	if existing != nil && isPluginEnabled(existing, plugin.PluginID) {
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
	if existing != nil {
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
	case strings.HasPrefix(action.Description, "plugin:claude:"):
		return i.applyClaudePlugin(action)
	case strings.HasPrefix(action.Description, "plugin:skill:"):
		return i.applySkillFile(action)
	default:
		return fmt.Errorf("unknown plugin action: %s", action.Description)
	}
}

// applyClaudePlugin enables a plugin in the adapter's settings.json.
func (i *Installer) applyClaudePlugin(action domain.PlannedAction) error {
	// Extract plugin name from description: "plugin:claude:<name>"
	name := strings.TrimPrefix(action.Description, "plugin:claude:")

	plugin, ok := i.plugins[name]
	if !ok {
		return fmt.Errorf("plugin %q not found in config", name)
	}

	existing, err := readJSONFile(action.TargetPath)
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	// Get or create enabledPlugins map.
	enabledPlugins, _ := existing["enabledPlugins"].(map[string]interface{})
	if enabledPlugins == nil {
		enabledPlugins = make(map[string]interface{})
	}
	enabledPlugins[plugin.PluginID] = true
	existing["enabledPlugins"] = enabledPlugins

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(action.TargetPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create settings dir: %w", err)
	}

	if _, err := fileutil.WriteAtomic(action.TargetPath, data, 0644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}

	return nil
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

	if len(i.plugins) == 0 {
		return nil, nil
	}

	var results []domain.VerifyResult
	agentID := string(adapter.ID())

	for _, name := range sortedPluginNames(i.plugins) {
		plugin := i.plugins[name]

		if !plugin.Enabled {
			continue
		}

		if i.config != nil && plugin.ExcludesMethodology != "" &&
			plugin.ExcludesMethodology == string(i.config.Methodology) {
			continue
		}

		if !isAgentSupported(agentID, plugin.SupportedAgents) {
			continue
		}

		switch effectiveInstallMethod(plugin, adapter) {
		case "claude_plugin":
			r := i.verifyClaudePlugin(adapter, name, plugin, homeDir)
			results = append(results, r)

		case "skill_files":
			r := i.verifySkillFile(adapter, name, projectDir)
			results = append(results, r)
		}
	}

	return results, nil
}

// verifyClaudePlugin checks that the plugin is enabled in settings.
func (i *Installer) verifyClaudePlugin(adapter domain.Adapter, name string, plugin domain.PluginDef, homeDir string) domain.VerifyResult {
	targetPath := adapter.SettingsPath(homeDir)
	checkName := fmt.Sprintf("plugin-%s-enabled", name)

	existing, err := readJSONFile(targetPath)
	if err != nil || existing == nil {
		return domain.VerifyResult{
			Check:     checkName,
			Passed:    false,
			Severity:  domain.SeverityError,
			Component: "plugins",
			Message:   fmt.Sprintf("settings file not found: %s", targetPath),
		}
	}

	if isPluginEnabled(existing, plugin.PluginID) {
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
		Message:   fmt.Sprintf("plugin %s not found in enabledPlugins", plugin.PluginID),
	}
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
	case strings.HasPrefix(action.Description, "plugin:claude:"):
		return i.renderClaudePluginContent(action)
	case strings.HasPrefix(action.Description, "plugin:skill:"):
		name := strings.TrimPrefix(action.Description, "plugin:skill:")
		return []byte(pluginSkillContentFor(name)), nil
	default:
		return nil, fmt.Errorf("unknown plugin action: %s", action.Description)
	}
}

// renderClaudePluginContent computes what applyClaudePlugin would write.
func (i *Installer) renderClaudePluginContent(action domain.PlannedAction) ([]byte, error) {
	name := strings.TrimPrefix(action.Description, "plugin:claude:")
	plugin, ok := i.plugins[name]
	if !ok {
		return nil, fmt.Errorf("plugin %q not found in config", name)
	}

	existing, err := readJSONFile(action.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("read settings: %w", err)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}

	enabledPlugins, _ := existing["enabledPlugins"].(map[string]interface{})
	if enabledPlugins == nil {
		enabledPlugins = make(map[string]interface{})
	}
	enabledPlugins[plugin.PluginID] = true
	existing["enabledPlugins"] = enabledPlugins

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal settings: %w", err)
	}
	data = append(data, '\n')
	return data, nil
}

// ─── Helpers ────────────────────────────────────────────────────────────────

// effectiveInstallMethod returns the install method to use for a plugin on a given adapter.
// claude_plugin is only valid for Claude Code; other agents fall back to skill_files.
func effectiveInstallMethod(plugin domain.PluginDef, adapter domain.Adapter) string {
	if plugin.InstallMethod == "claude_plugin" && adapter.ID() != domain.AgentClaudeCode {
		return "skill_files"
	}
	return plugin.InstallMethod
}

// isAgentSupported checks if agentID is in the supported agents list.
func isAgentSupported(agentID string, supported []string) bool {
	for _, s := range supported {
		if s == agentID {
			return true
		}
	}
	return false
}

// isPluginEnabled checks if a plugin ID is enabled in the settings document.
func isPluginEnabled(doc map[string]interface{}, pluginID string) bool {
	plugins, ok := doc["enabledPlugins"].(map[string]interface{})
	if !ok {
		return false
	}
	val, ok := plugins[pluginID]
	if !ok {
		return false
	}
	enabled, ok := val.(bool)
	return ok && enabled
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

// readJSONFile reads a JSON file into a generic map.
// Returns nil, nil if the file does not exist.
func readJSONFile(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	if len(data) == 0 {
		return nil, nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return result, nil
}
