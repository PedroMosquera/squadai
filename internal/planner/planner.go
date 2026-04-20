package planner

import (
	"fmt"
	"os"

	"github.com/PedroMosquera/squadai/internal/components/agents"
	"github.com/PedroMosquera/squadai/internal/components/bundle"
	"github.com/PedroMosquera/squadai/internal/components/commands"
	"github.com/PedroMosquera/squadai/internal/components/copilot"
	"github.com/PedroMosquera/squadai/internal/components/mcp"
	"github.com/PedroMosquera/squadai/internal/components/memory"
	"github.com/PedroMosquera/squadai/internal/components/permissions"
	"github.com/PedroMosquera/squadai/internal/components/plugins"
	"github.com/PedroMosquera/squadai/internal/components/rules"
	"github.com/PedroMosquera/squadai/internal/components/settings"
	"github.com/PedroMosquera/squadai/internal/components/skills"
	"github.com/PedroMosquera/squadai/internal/components/workflows"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// Planner computes the full action plan from merged config and detected adapters.
type Planner struct {
	memoryInstaller      *memory.Installer
	rulesInstaller       *rules.Installer
	settingsInstaller    *settings.Installer
	mcpInstaller         *mcp.Installer
	agentsInstaller      *agents.Installer
	skillsInstaller      *skills.Installer
	commandsInstaller    *commands.Installer
	pluginsInstaller     *plugins.Installer
	workflowsInstaller   *workflows.Installer
	permissionsInstaller *permissions.Installer
	copilotManager       *copilot.Manager
	opts                 Options
}

// Options controls optional behavior passed to component installers.
type Options struct {
	// SetClaudeDefaultAgent passes the corresponding option to the agents installer.
	SetClaudeDefaultAgent bool
}

// New returns a Planner with default component installers.
func New(opts ...Options) *Planner {
	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	return &Planner{
		opts: o,
	}
}

// loadFromSet wires the built Set into the planner's installer fields. Kept
// private: callers should construct the planner via New() and let Plan build
// the set, or use FromSet for test doubles.
func (p *Planner) loadFromSet(s *bundle.Set) {
	p.memoryInstaller = s.Memory
	p.rulesInstaller = s.Rules
	p.settingsInstaller = s.Settings
	p.permissionsInstaller = s.Permissions
	p.mcpInstaller = s.MCP
	p.agentsInstaller = s.Agents
	p.skillsInstaller = s.Skills
	p.commandsInstaller = s.Commands
	p.pluginsInstaller = s.Plugins
	p.workflowsInstaller = s.Workflows
	p.copilotManager = s.Copilot
}

// Plan returns the ordered list of actions needed to reach the desired state.
// It iterates over enabled adapters and components, collecting actions from
// each component installer, then appends copilot instructions if configured.
func (p *Planner) Plan(cfg *domain.MergedConfig, adapters []domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	var actions []domain.PlannedAction

	// Build the full installer set via the shared bundle builder so the
	// planner and verifier cannot drift in how they instantiate components.
	set, err := bundle.Build(cfg, projectDir, bundle.Options{
		SetClaudeDefaultAgent: p.opts.SetClaudeDefaultAgent,
	})
	if err != nil {
		return nil, err
	}
	p.loadFromSet(set)

	// Collect component actions for each enabled adapter.
	for _, adapter := range adapters {
		adapterCfg, ok := cfg.Adapters[string(adapter.ID())]
		if !ok || !adapterCfg.Enabled {
			continue
		}

		// Memory component.
		if memCfg, ok := cfg.Components[string(domain.ComponentMemory)]; ok && memCfg.Enabled {
			memActions, err := p.memoryInstaller.Plan(adapter, homeDir, projectDir)
			if err != nil {
				return nil, fmt.Errorf("plan memory for %s: %w", adapter.ID(), err)
			}
			actions = append(actions, memActions...)
		}

		// Rules component.
		if rulesCfg, ok := cfg.Components[string(domain.ComponentRules)]; ok && rulesCfg.Enabled {
			rulesActions, err := p.rulesInstaller.Plan(adapter, homeDir, projectDir)
			if err != nil {
				return nil, fmt.Errorf("plan rules for %s: %w", adapter.ID(), err)
			}
			actions = append(actions, rulesActions...)
		}

		// Settings component.
		if settingsCfg, ok := cfg.Components[string(domain.ComponentSettings)]; ok && settingsCfg.Enabled {
			settingsActions, err := p.settingsInstaller.Plan(adapter, homeDir, projectDir)
			if err != nil {
				return nil, fmt.Errorf("plan settings for %s: %w", adapter.ID(), err)
			}
			actions = append(actions, settingsActions...)
		}

		// Permissions component (runs after settings so the file exists first).
		if permCfg, ok := cfg.Components[string(domain.ComponentPermissions)]; ok && permCfg.Enabled {
			permActions, err := p.permissionsInstaller.Plan(adapter, homeDir, projectDir)
			if err != nil {
				return nil, fmt.Errorf("plan permissions for %s: %w", adapter.ID(), err)
			}
			actions = append(actions, permActions...)
		}

		// MCP component.
		if mcpCfg, ok := cfg.Components[string(domain.ComponentMCP)]; ok && mcpCfg.Enabled {
			mcpActions, err := p.mcpInstaller.Plan(adapter, homeDir, projectDir)
			if err != nil {
				return nil, fmt.Errorf("plan mcp for %s: %w", adapter.ID(), err)
			}
			actions = append(actions, mcpActions...)
		}

		// Agents component.
		if agentsCfg, ok := cfg.Components[string(domain.ComponentAgents)]; ok && agentsCfg.Enabled {
			agentsActions, err := p.agentsInstaller.Plan(adapter, homeDir, projectDir)
			if err != nil {
				return nil, fmt.Errorf("plan agents for %s: %w", adapter.ID(), err)
			}
			actions = append(actions, agentsActions...)
		}

		// Skills component.
		if skillsCfg, ok := cfg.Components[string(domain.ComponentSkills)]; ok && skillsCfg.Enabled {
			skillsActions, err := p.skillsInstaller.Plan(adapter, homeDir, projectDir)
			if err != nil {
				return nil, fmt.Errorf("plan skills for %s: %w", adapter.ID(), err)
			}
			actions = append(actions, skillsActions...)
		}

		// Commands component.
		if cmdsCfg, ok := cfg.Components[string(domain.ComponentCommands)]; ok && cmdsCfg.Enabled {
			cmdsActions, err := p.commandsInstaller.Plan(adapter, homeDir, projectDir)
			if err != nil {
				return nil, fmt.Errorf("plan commands for %s: %w", adapter.ID(), err)
			}
			actions = append(actions, cmdsActions...)
		}

		// Plugins component.
		if pluginsCfg, ok := cfg.Components[string(domain.ComponentPlugins)]; ok && pluginsCfg.Enabled {
			pluginsActions, err := p.pluginsInstaller.Plan(adapter, homeDir, projectDir)
			if err != nil {
				return nil, fmt.Errorf("plugins plan: %w", err)
			}
			actions = append(actions, pluginsActions...)
		}

		// Workflows component.
		if workflowsCfg, ok := cfg.Components[string(domain.ComponentWorkflows)]; ok && workflowsCfg.Enabled {
			workflowsActions, err := p.workflowsInstaller.Plan(adapter, homeDir, projectDir)
			if err != nil {
				return nil, fmt.Errorf("workflows plan: %w", err)
			}
			actions = append(actions, workflowsActions...)
		}
	}

	// Copilot instructions (project-level, not adapter-specific).
	if cfg.Copilot.InstructionsTemplate != "" {
		copilotAction, err := p.copilotManager.Plan(projectDir, cfg.Copilot)
		if err != nil {
			return nil, fmt.Errorf("plan copilot instructions: %w", err)
		}
		actions = append(actions, copilotAction)
	}

	// Stale file cleanup pass: for adapters that are disabled, emit ActionDelete
	// for any managed files that still exist on disk.
	deleteActions := p.planStaleCleanup(cfg, adapters, homeDir, projectDir)
	actions = append(actions, deleteActions...)

	return actions, nil
}

// planStaleCleanup returns ActionDelete actions for files belonging to disabled
// adapters that still exist on disk. This prevents orphaned managed files when
// an adapter transitions from enabled to disabled.
func (p *Planner) planStaleCleanup(cfg *domain.MergedConfig, adapters []domain.Adapter, homeDir, projectDir string) []domain.PlannedAction {
	// Build a lookup of adapters by ID for matching against cfg.Adapters.
	adapterByID := make(map[domain.AgentID]domain.Adapter, len(adapters))
	for _, adapter := range adapters {
		adapterByID[adapter.ID()] = adapter
	}

	var deleteActions []domain.PlannedAction

	for adapterKey, adapterCfg := range cfg.Adapters {
		if adapterCfg.Enabled {
			continue // only clean up disabled adapters
		}

		adapter, ok := adapterByID[domain.AgentID(adapterKey)]
		if !ok {
			continue // adapter not in the provided list — nothing to clean up
		}

		// Collect all file paths that this adapter's components can write to.
		paths := managedFilePaths(adapter, homeDir, projectDir)

		for _, path := range paths {
			if path == "" {
				continue
			}
			if _, err := os.Stat(path); err != nil {
				continue // file does not exist — nothing to delete
			}
			deleteActions = append(deleteActions, domain.PlannedAction{
				ID:          fmt.Sprintf("%s-stale-cleanup-%s", adapterKey, sanitizePath(path)),
				Agent:       adapter.ID(),
				Component:   domain.ComponentCleanup,
				Action:      domain.ActionDelete,
				TargetPath:  path,
				Description: fmt.Sprintf("remove stale file for disabled adapter %s: %s", adapterKey, path),
			})
		}
	}

	return deleteActions
}

// managedFilePaths returns the set of individual file paths that an adapter's
// components may write to. Directory paths (agents dir, skills dir, etc.) are
// excluded because their contents are not directly managed as single files.
func managedFilePaths(adapter domain.Adapter, homeDir, projectDir string) []string {
	return []string{
		adapter.SystemPromptFile(homeDir),
		adapter.SettingsPath(homeDir),
		adapter.ProjectRulesFile(projectDir),
		adapter.ProjectConfigFile(projectDir),
	}
}

// sanitizePath converts a file path to a string safe for use in an action ID
// by replacing path separators with dashes.
func sanitizePath(path string) string {
	result := make([]byte, len(path))
	for i := 0; i < len(path); i++ {
		if path[i] == '/' || path[i] == '\\' || path[i] == ':' {
			result[i] = '-'
		} else {
			result[i] = path[i]
		}
	}
	return string(result)
}

// ComponentInstallers returns the installers used by this planner.
// This is used by the executor to delegate Apply calls.
func (p *Planner) ComponentInstallers() map[domain.ComponentID]domain.ComponentInstaller {
	installers := map[domain.ComponentID]domain.ComponentInstaller{
		domain.ComponentMemory: p.memoryInstaller,
	}
	if p.rulesInstaller != nil {
		installers[domain.ComponentRules] = p.rulesInstaller
	}
	if p.settingsInstaller != nil {
		installers[domain.ComponentSettings] = p.settingsInstaller
	}
	if p.mcpInstaller != nil {
		installers[domain.ComponentMCP] = p.mcpInstaller
	}
	if p.agentsInstaller != nil {
		installers[domain.ComponentAgents] = p.agentsInstaller
	}
	if p.skillsInstaller != nil {
		installers[domain.ComponentSkills] = p.skillsInstaller
	}
	if p.commandsInstaller != nil {
		installers[domain.ComponentCommands] = p.commandsInstaller
	}
	if p.pluginsInstaller != nil {
		installers[domain.ComponentPlugins] = p.pluginsInstaller
	}
	if p.workflowsInstaller != nil {
		installers[domain.ComponentWorkflows] = p.workflowsInstaller
	}
	if p.permissionsInstaller != nil {
		installers[domain.ComponentPermissions] = p.permissionsInstaller
	}
	return installers
}

// CopilotManager returns the copilot manager for the executor.
func (p *Planner) CopilotManager() *copilot.Manager {
	return p.copilotManager
}

// RenderAction returns the old and new content for a planned action.
// This enables diff display without writing files.
// Must be called AFTER Plan() (which initializes the component installers).
func (p *Planner) RenderAction(action domain.PlannedAction, homeDir, projectDir string) (oldContent, newContent []byte, err error) {
	// Read existing file content (old state).
	existing, readErr := fileutil.ReadFileOrEmpty(action.TargetPath)
	if readErr != nil {
		return nil, nil, fmt.Errorf("read existing file: %w", readErr)
	}
	oldContent = existing

	// For delete actions there is no new content.
	if action.Action == domain.ActionDelete {
		return oldContent, nil, nil
	}

	// Copilot instructions are handled by the copilot manager.
	if action.ID == "copilot-instructions" {
		return p.renderCopilot(action, oldContent, projectDir)
	}

	// Route by component.
	switch action.Component {
	case domain.ComponentMemory:
		return p.renderMemory(action, oldContent)

	case domain.ComponentRules:
		return p.renderRules(action, oldContent)

	case domain.ComponentSkills:
		return p.renderSkills(action, oldContent)

	case domain.ComponentCommands:
		return p.renderCommands(action, oldContent)

	case domain.ComponentAgents:
		return p.renderAgents(action, oldContent)

	case domain.ComponentMCP:
		return p.renderMCP(action, oldContent)

	case domain.ComponentSettings:
		return p.renderSettings(action, oldContent)

	case domain.ComponentPlugins:
		return p.renderPlugins(action, oldContent)

	case domain.ComponentWorkflows:
		return p.renderWorkflows(action, oldContent)

	case domain.ComponentPermissions:
		return p.renderPermissions(action, oldContent)

	default:
		return oldContent, []byte("[content preview not available for " + string(action.Component) + "]"), nil
	}
}

// renderMemory computes what the memory installer would write.
func (p *Planner) renderMemory(action domain.PlannedAction, existing []byte) ([]byte, []byte, error) {
	if p.memoryInstaller == nil {
		return existing, []byte("[memory installer not initialized]"), nil
	}
	content := memory.TemplateForAgentID(action.Agent)
	updated := injectSection(string(existing), memory.SectionID, content)
	return existing, []byte(updated), nil
}

// renderRules computes what the rules installer would write.
func (p *Planner) renderRules(action domain.PlannedAction, existing []byte) ([]byte, []byte, error) {
	if p.rulesInstaller == nil {
		return existing, []byte("[rules installer not initialized]"), nil
	}
	content := p.rulesInstaller.Content()
	if content == "" {
		return existing, existing, nil
	}
	updated := injectSection(string(existing), rules.SectionID, content)
	return existing, []byte(updated), nil
}

// renderSkills computes what the skills installer would write.
func (p *Planner) renderSkills(action domain.PlannedAction, existing []byte) ([]byte, []byte, error) {
	if p.skillsInstaller == nil {
		return existing, []byte("[skills installer not initialized]"), nil
	}
	newContent, err := p.skillsInstaller.RenderContent(action)
	if err != nil {
		return existing, []byte("[content preview not available for skills: " + err.Error() + "]"), nil
	}
	return existing, []byte(newContent), nil
}

// renderCommands computes what the commands installer would write.
func (p *Planner) renderCommands(action domain.PlannedAction, existing []byte) ([]byte, []byte, error) {
	if p.commandsInstaller == nil {
		return existing, []byte("[commands installer not initialized]"), nil
	}
	newContent, err := p.commandsInstaller.RenderContent(action)
	if err != nil {
		return existing, []byte("[content preview not available for commands: " + err.Error() + "]"), nil
	}
	return existing, []byte(newContent), nil
}

// renderAgents computes what the agents installer would write.
func (p *Planner) renderAgents(action domain.PlannedAction, existing []byte) ([]byte, []byte, error) {
	if p.agentsInstaller == nil {
		return existing, []byte("[agents installer not initialized]"), nil
	}
	newContent, err := p.agentsInstaller.RenderContent(action)
	if err != nil {
		return existing, []byte("[content preview not available for agents: " + err.Error() + "]"), nil
	}
	if newContent == "" {
		return existing, existing, nil
	}
	return existing, []byte(newContent), nil
}

// renderMCP computes what the MCP installer would write.
func (p *Planner) renderMCP(action domain.PlannedAction, existing []byte) ([]byte, []byte, error) {
	if p.mcpInstaller == nil {
		return existing, []byte("[mcp installer not initialized]"), nil
	}
	newContent, err := p.mcpInstaller.RenderContent(action)
	if err != nil {
		return existing, []byte("[content preview not available for mcp: " + err.Error() + "]"), nil
	}
	return existing, []byte(newContent), nil
}

// renderSettings computes what the settings installer would write.
func (p *Planner) renderSettings(action domain.PlannedAction, existing []byte) ([]byte, []byte, error) {
	if p.settingsInstaller == nil {
		return existing, []byte("[settings installer not initialized]"), nil
	}
	newContent, err := p.settingsInstaller.RenderContent(action)
	if err != nil {
		return existing, []byte("[content preview not available for settings: " + err.Error() + "]"), nil
	}
	return existing, []byte(newContent), nil
}

// renderPlugins computes what the plugins installer would write.
func (p *Planner) renderPlugins(action domain.PlannedAction, existing []byte) ([]byte, []byte, error) {
	if p.pluginsInstaller == nil {
		return existing, []byte("[plugins installer not initialized]"), nil
	}
	newContent, err := p.pluginsInstaller.RenderContent(action)
	if err != nil {
		return existing, []byte("[content preview not available for plugins: " + err.Error() + "]"), nil
	}
	return existing, []byte(newContent), nil
}

// renderWorkflows computes what the workflows installer would write.
func (p *Planner) renderWorkflows(action domain.PlannedAction, existing []byte) ([]byte, []byte, error) {
	if p.workflowsInstaller == nil {
		return existing, []byte("[workflows installer not initialized]"), nil
	}
	newContent, err := p.workflowsInstaller.RenderContent(action)
	if err != nil {
		return existing, []byte("[content preview not available for workflows: " + err.Error() + "]"), nil
	}
	return existing, []byte(newContent), nil
}

// renderPermissions computes what the permissions installer would write.
func (p *Planner) renderPermissions(action domain.PlannedAction, existing []byte) ([]byte, []byte, error) {
	if p.permissionsInstaller == nil {
		return existing, []byte("[permissions installer not initialized]"), nil
	}
	newContent, err := p.permissionsInstaller.RenderContent(action)
	if err != nil {
		return existing, []byte("[content preview not available for permissions: " + err.Error() + "]"), nil
	}
	return existing, newContent, nil
}

// renderCopilot computes what the copilot manager would write.
func (p *Planner) renderCopilot(action domain.PlannedAction, existing []byte, projectDir string) ([]byte, []byte, error) {
	_ = action
	// We don't have the copilot config here directly; use the manager's logic via Apply
	// but without writing. Since copilot config is embedded in action ID pattern,
	// use a fallback preview message.
	return existing, []byte("[content preview not available for copilot-instructions]"), nil
}

// injectSection delegates to marker.InjectSection for inserting or replacing
// managed content between squadai marker tags in a document.
func injectSection(document, sectionID, content string) string {
	return marker.InjectSection(document, sectionID, content)
}
