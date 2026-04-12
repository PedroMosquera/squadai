package planner

import (
	"fmt"

	"github.com/PedroMosquera/agent-manager-pro/internal/components/copilot"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/memory"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// Planner computes the full action plan from merged config and detected adapters.
type Planner struct {
	memoryInstaller *memory.Installer
	copilotManager  *copilot.Manager
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
	}

	// Copilot instructions (project-level, not adapter-specific).
	if cfg.Copilot.InstructionsTemplate != "" {
		copilotAction, err := p.copilotManager.Plan(projectDir, cfg.Copilot.InstructionsTemplate)
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
	return map[domain.ComponentID]domain.ComponentInstaller{
		domain.ComponentMemory: p.memoryInstaller,
	}
}

// CopilotManager returns the copilot manager for the executor.
func (p *Planner) CopilotManager() *copilot.Manager {
	return p.copilotManager
}
