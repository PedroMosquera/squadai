package planner

import (
	"fmt"

	"github.com/PedroMosquera/agent-manager-pro/internal/components/agents"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/commands"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/copilot"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/mcp"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/memory"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/rules"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/settings"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/skills"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// Planner computes the full action plan from merged config and detected adapters.
type Planner struct {
	memoryInstaller   *memory.Installer
	rulesInstaller    *rules.Installer
	settingsInstaller *settings.Installer
	mcpInstaller      *mcp.Installer
	agentsInstaller   *agents.Installer
	skillsInstaller   *skills.Installer
	commandsInstaller *commands.Installer
	copilotManager    *copilot.Manager
}

// New returns a Planner with default component installers.
func New() *Planner {
	return &Planner{
		memoryInstaller: memory.New(),
		copilotManager:  copilot.New(),
	}
}

// Plan returns the ordered list of actions needed to reach the desired state.
// It iterates over enabled adapters and components, collecting actions from
// each component installer, then appends copilot instructions if configured.
func (p *Planner) Plan(cfg *domain.MergedConfig, adapters []domain.Adapter, homeDir, projectDir string) ([]domain.PlannedAction, error) {
	var actions []domain.PlannedAction

	// Create rules installer from merged config (lazy init per plan call).
	p.rulesInstaller = rules.New(cfg.Rules, projectDir)

	// Create settings installer from merged adapter configs (lazy init per plan call).
	p.settingsInstaller = settings.New(cfg.Adapters)

	// Create MCP installer from merged MCP config (lazy init per plan call).
	p.mcpInstaller = mcp.New(cfg.MCP)

	// Create agents/skills/commands installers (lazy init per plan call).
	p.agentsInstaller = agents.New(cfg.Agents, cfg, projectDir)
	p.skillsInstaller = skills.New(cfg.Skills, cfg, projectDir)
	p.commandsInstaller = commands.New(cfg.Commands)

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
	}

	// Copilot instructions (project-level, not adapter-specific).
	if cfg.Copilot.InstructionsTemplate != "" {
		copilotAction, err := p.copilotManager.Plan(projectDir, cfg.Copilot)
		if err != nil {
			return nil, fmt.Errorf("plan copilot instructions: %w", err)
		}
		actions = append(actions, copilotAction)
	}

	return actions, nil
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
	return installers
}

// CopilotManager returns the copilot manager for the executor.
func (p *Planner) CopilotManager() *copilot.Manager {
	return p.copilotManager
}
