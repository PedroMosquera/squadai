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

	"github.com/PedroMosquera/squadai/internal/components/agent_teams"
	"github.com/PedroMosquera/squadai/internal/components/agents"
	"github.com/PedroMosquera/squadai/internal/components/brand"
	"github.com/PedroMosquera/squadai/internal/components/commands"
	"github.com/PedroMosquera/squadai/internal/components/copilot"
	"github.com/PedroMosquera/squadai/internal/components/efficiency"
	"github.com/PedroMosquera/squadai/internal/components/hooks"
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
	AgentTeams  *agent_teams.Installer
	Hooks       *hooks.Installer
	Brand       *brand.Installer
	Efficiency  *efficiency.Installer
	Copilot     *copilot.Manager
}

// MemoryEnabled reports whether the memory component is enabled in cfg.
func MemoryEnabled(cfg *domain.MergedConfig) bool {
	c, ok := cfg.Components[string(domain.ComponentMemory)]
	return ok && c.Enabled
}

// EfficiencyEnabled reports whether the efficiency component is enabled.
// Unlike other components it defaults to ON when the key is absent, so
// projects created before the component existed gain it on their next apply.
func EfficiencyEnabled(cfg *domain.MergedConfig) bool {
	if c, ok := cfg.Components[string(domain.ComponentEfficiency)]; ok {
		return c.Enabled
	}
	return true
}

// Build returns a Set with installers for every enabled component plus the
// always-present memory installer and copilot manager.
func Build(cfg *domain.MergedConfig, projectDir string, opts Options) (*Set, error) {
	profile := cfg.ActiveContextProfile

	memOpts := memory.Options{}
	if profile != nil {
		memOpts.Scope = profile.MemoryScope
	}

	s := &Set{
		Memory:      memory.New(memOpts),
		Copilot:     copilot.New(),
		Permissions: permissions.New(),
		Brand:       brand.New(),
		Efficiency:  efficiency.New(efficiency.Options{MemoryEnabled: MemoryEnabled(cfg)}),
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
		// When a context profile declares an MCP filter (even an empty one),
		// the installer must prune previously managed servers that were
		// filtered out — including down to an empty set.
		s.MCP = mcp.New(cfg.MCP, mcp.Options{
			PruneWhenEmpty: profile != nil && profile.MCPServers != nil,
		})
	}

	if agentsCfg, ok := cfg.Components[string(domain.ComponentAgents)]; ok && agentsCfg.Enabled {
		s.Agents = agents.New(cfg.Agents, cfg, projectDir,
			agents.Options{SetClaudeDefaultAgent: opts.SetClaudeDefaultAgent})
	}

	if skillsCfg, ok := cfg.Components[string(domain.ComponentSkills)]; ok && skillsCfg.Enabled {
		skillOpts := skills.Options{}
		if profile != nil {
			skillOpts.Scopes = profile.SkillScopes
		}
		s.Skills = skills.New(cfg.Skills, cfg, projectDir, skillOpts)
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

	// Agent Teams runs whenever Claude is an enabled adapter — it has to fire
	// even when the desired state is "disabled" so toggling off triggers a
	// teardown action rather than leaving stale env vars in settings.json.
	if claudeCfg, ok := cfg.Adapters[string(domain.AgentClaudeCode)]; ok && claudeCfg.Enabled {
		s.AgentTeams = agent_teams.New(cfg.Claude.AgentTeams.Enabled)
	}

	// Hooks installer runs whenever Claude is enabled and hooks are configured.
	if claudeCfg, ok := cfg.Adapters[string(domain.AgentClaudeCode)]; ok && claudeCfg.Enabled {
		if len(cfg.Hooks) > 0 {
			s.Hooks = hooks.New(cfg.Hooks)
		}
	}

	return s, nil
}
