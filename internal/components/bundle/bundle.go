// Package bundle builds the complete set of component installers from a
// merged config. It is the single source of truth for installer construction
// so that the planner and the verifier cannot drift in how they instantiate
// components.
//
// The builder is pure: each Build call returns a fresh Set. Callers that
// need the same instances across both plan and verify (e.g., to preserve an
// ApplyPolicy set on a component) must pass the same Set to both calls.
package bundle

import (
	"fmt"

	"github.com/PedroMosquera/squadai/internal/components/agents"
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
)

// Options threads through to the agents installer and any other
// construction-time knobs.
type Options struct {
	SetClaudeDefaultAgent bool
}

// Set holds every component installer + copilot manager. Fields are nil when
// the corresponding component is disabled in the merged config.
type Set struct {
	Memory      *memory.Installer
	Rules       *rules.Installer
	Settings    *settings.Installer
	Permissions *permissions.Installer
	MCP         *mcp.Installer
	Agents      *agents.Installer
	Skills      *skills.Installer
	Commands    *commands.Installer
	Plugins     *plugins.Installer
	Workflows   *workflows.Installer
	Copilot     *copilot.Manager
}

// Build returns a Set with installers for every enabled component plus the
// always-present memory installer and copilot manager.
func Build(cfg *domain.MergedConfig, projectDir string, opts Options) (*Set, error) {
	s := &Set{
		Memory:      memory.New(),
		Copilot:     copilot.New(),
		Permissions: permissions.New(),
	}

	if rulesCfg, ok := cfg.Components[string(domain.ComponentRules)]; ok && rulesCfg.Enabled {
		ri, err := rules.New(cfg.Rules, projectDir)
		if err != nil {
			return nil, fmt.Errorf("create rules installer: %w", err)
		}
		s.Rules = ri
	}

	if settingsCfg, ok := cfg.Components[string(domain.ComponentSettings)]; ok && settingsCfg.Enabled {
		s.Settings = settings.New(cfg.Adapters)
	}

	if mcpCfg, ok := cfg.Components[string(domain.ComponentMCP)]; ok && mcpCfg.Enabled {
		s.MCP = mcp.New(cfg.MCP)
	}

	if agentsCfg, ok := cfg.Components[string(domain.ComponentAgents)]; ok && agentsCfg.Enabled {
		s.Agents = agents.New(cfg.Agents, cfg, projectDir,
			agents.Options{SetClaudeDefaultAgent: opts.SetClaudeDefaultAgent})
	}

	if skillsCfg, ok := cfg.Components[string(domain.ComponentSkills)]; ok && skillsCfg.Enabled {
		s.Skills = skills.New(cfg.Skills, cfg, projectDir)
	}

	if cmdsCfg, ok := cfg.Components[string(domain.ComponentCommands)]; ok && cmdsCfg.Enabled {
		s.Commands = commands.New(cfg.Commands)
	}

	if pluginsCfg, ok := cfg.Components[string(domain.ComponentPlugins)]; ok && pluginsCfg.Enabled {
		s.Plugins = plugins.New(cfg.Plugins, cfg)
	}

	if workflowsCfg, ok := cfg.Components[string(domain.ComponentWorkflows)]; ok && workflowsCfg.Enabled {
		s.Workflows = workflows.New(cfg)
	}

	return s, nil
}
