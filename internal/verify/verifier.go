package verify

import (
	"context"
	"fmt"
	"sort"

	"github.com/PedroMosquera/squadai/internal/components/agents"
	"github.com/PedroMosquera/squadai/internal/components/commands"
	"github.com/PedroMosquera/squadai/internal/components/copilot"
	"github.com/PedroMosquera/squadai/internal/components/mcp"
	"github.com/PedroMosquera/squadai/internal/components/memory"
	"github.com/PedroMosquera/squadai/internal/components/plugins"
	"github.com/PedroMosquera/squadai/internal/components/rules"
	"github.com/PedroMosquera/squadai/internal/components/settings"
	"github.com/PedroMosquera/squadai/internal/components/skills"
	"github.com/PedroMosquera/squadai/internal/components/workflows"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// Verifier runs post-apply compliance checks across all components and adapters.
type Verifier struct {
	memoryInstaller *memory.Installer
	copilotManager  *copilot.Manager
}

// New returns a Verifier with default component checkers.
func New() *Verifier {
	return &Verifier{
		memoryInstaller: memory.New(),
		copilotManager:  copilot.New(),
	}
}

// Verify runs all checks and produces a report.
func (v *Verifier) Verify(cfg *domain.MergedConfig, adapters []domain.Adapter, homeDir, projectDir string) (*domain.VerifyReport, error) {
	report := &domain.VerifyReport{
		AllPass: true,
	}

	// Create rules installer from merged config (lazy init per verify call).
	var rulesInstaller *rules.Installer
	if rulesCfg, ok := cfg.Components[string(domain.ComponentRules)]; ok && rulesCfg.Enabled {
		ri, err := rules.New(cfg.Rules, projectDir)
		if err != nil {
			return nil, fmt.Errorf("create rules installer: %w", err)
		}
		rulesInstaller = ri
	}

	// Create settings installer from merged adapter configs (lazy init per verify call).
	var settingsInstaller *settings.Installer
	if settingsCfg, ok := cfg.Components[string(domain.ComponentSettings)]; ok && settingsCfg.Enabled {
		settingsInstaller = settings.New(cfg.Adapters)
	}

	// Create MCP installer from merged MCP config (lazy init per verify call).
	var mcpInstaller *mcp.Installer
	if mcpCfg, ok := cfg.Components[string(domain.ComponentMCP)]; ok && mcpCfg.Enabled {
		mcpInstaller = mcp.New(cfg.MCP)
	}

	// Create agents installer from merged config (lazy init per verify call).
	var agentsInstaller *agents.Installer
	if agentsCfg, ok := cfg.Components[string(domain.ComponentAgents)]; ok && agentsCfg.Enabled {
		agentsInstaller = agents.New(cfg.Agents, cfg, projectDir)
	}

	// Create skills installer from merged config (lazy init per verify call).
	var skillsInstaller *skills.Installer
	if skillsCfg, ok := cfg.Components[string(domain.ComponentSkills)]; ok && skillsCfg.Enabled {
		skillsInstaller = skills.New(cfg.Skills, cfg, projectDir)
	}

	// Create commands installer from merged config (lazy init per verify call).
	var commandsInstaller *commands.Installer
	if cmdsCfg, ok := cfg.Components[string(domain.ComponentCommands)]; ok && cmdsCfg.Enabled {
		commandsInstaller = commands.New(cfg.Commands)
	}

	// Create plugins installer from merged config (lazy init per verify call).
	var pluginsInstaller *plugins.Installer
	if pluginsCfg, ok := cfg.Components[string(domain.ComponentPlugins)]; ok && pluginsCfg.Enabled {
		pluginsInstaller = plugins.New(cfg.Plugins, cfg)
	}

	// Create workflows installer from merged config (lazy init per verify call).
	var workflowsInstaller *workflows.Installer
	if workflowsCfg, ok := cfg.Components[string(domain.ComponentWorkflows)]; ok && workflowsCfg.Enabled {
		workflowsInstaller = workflows.New(cfg)
	}

	// Verify components for each enabled adapter.
	for _, adapter := range adapters {
		adapterCfg, ok := cfg.Adapters[string(adapter.ID())]
		if !ok || !adapterCfg.Enabled {
			continue
		}

		// Memory component.
		if memCfg, ok := cfg.Components[string(domain.ComponentMemory)]; ok && memCfg.Enabled {
			results, err := v.memoryInstaller.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "memory")
			collectResults(report, results)
		}

		// Rules component.
		if rulesInstaller != nil {
			results, err := rulesInstaller.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "rules")
			collectResults(report, results)
		}

		// Settings component.
		if settingsInstaller != nil {
			results, err := settingsInstaller.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "settings")
			collectResults(report, results)
		}

		// MCP component.
		if mcpInstaller != nil {
			results, err := mcpInstaller.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "mcp")
			collectResults(report, results)
		}

		// Agents component.
		if agentsInstaller != nil {
			results, err := agentsInstaller.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "agents")
			collectResults(report, results)
		}

		// Skills component.
		if skillsInstaller != nil {
			results, err := skillsInstaller.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "skills")
			collectResults(report, results)
		}

		// Commands component.
		if commandsInstaller != nil {
			results, err := commandsInstaller.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "commands")
			collectResults(report, results)
		}

		// Plugins component.
		if pluginsInstaller != nil {
			results, err := pluginsInstaller.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "plugins")
			collectResults(report, results)
		}

		// Workflows component.
		if workflowsInstaller != nil {
			results, err := workflowsInstaller.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "workflows")
			collectResults(report, results)
		}
	}

	// Verify copilot instructions.
	if cfg.Copilot.InstructionsTemplate != "" {
		results := v.copilotManager.Verify(projectDir, cfg.Copilot)
		tagResults(results, "copilot")
		collectResults(report, results)
	}

	// Agent health checks — report configured vs. detected adapter status.
	healthResults := v.checkAgentHealth(cfg, adapters, homeDir)
	collectResults(report, healthResults)

	// Report policy violations as warnings.
	if len(cfg.Violations) > 0 {
		for _, violation := range cfg.Violations {
			report.Results = append(report.Results, domain.VerifyResult{
				Check:     "policy-override",
				Passed:    true, // violations are informational — policy value won
				Severity:  domain.SeverityWarning,
				Component: "policy",
				Message:   violation,
			})
		}
	}

	return report, nil
}

// checkAgentHealth verifies that configured adapters are detected and reports
// any detected-but-unconfigured agents as informational warnings.
func (v *Verifier) checkAgentHealth(cfg *domain.MergedConfig, adapters []domain.Adapter, homeDir string) []domain.VerifyResult {
	var results []domain.VerifyResult

	// Build set of detected adapter IDs.
	detected := make(map[string]bool)
	for _, a := range adapters {
		detected[string(a.ID())] = true
	}

	// Check each configured adapter is detected (binary or config present).
	configuredIDs := sortedMapKeys(cfg.Adapters)
	for _, id := range configuredIDs {
		acfg := cfg.Adapters[id]
		if !acfg.Enabled {
			continue
		}

		if detected[id] {
			results = append(results, domain.VerifyResult{
				Check:     fmt.Sprintf("agent-%s-detected", id),
				Passed:    true,
				Severity:  domain.SeverityInfo,
				Component: "health",
				Message:   fmt.Sprintf("agent %s is installed and configured", id),
			})
		} else {
			results = append(results, domain.VerifyResult{
				Check:     fmt.Sprintf("agent-%s-detected", id),
				Passed:    false,
				Severity:  domain.SeverityError,
				Component: "health",
				Message:   fmt.Sprintf("agent %s is configured but not detected on this system", id),
			})
		}
	}

	// Check for detected-but-unconfigured agents.
	for _, a := range adapters {
		id := string(a.ID())
		acfg, ok := cfg.Adapters[id]
		if !ok || !acfg.Enabled {
			// Detected but not configured — report as warning.
			// Verify detection is genuine (binary or config exists).
			ctx := context.Background()
			installed, configFound, err := a.Detect(ctx, homeDir)
			if err != nil || (!installed && !configFound) {
				continue // detection is uncertain, skip
			}
			results = append(results, domain.VerifyResult{
				Check:     fmt.Sprintf("agent-%s-unconfigured", id),
				Passed:    true, // not a failure — informational
				Severity:  domain.SeverityWarning,
				Component: "health",
				Message:   fmt.Sprintf("agent %s is detected but not configured", id),
			})
		}
	}

	return results
}

// tagResults fills in Severity and Component on results that don't have them set.
// Severity defaults to "error" for failed and "info" for passed.
func tagResults(results []domain.VerifyResult, component string) {
	for i := range results {
		if results[i].Component == "" {
			results[i].Component = component
		}
		if results[i].Severity == "" {
			if results[i].Passed {
				results[i].Severity = domain.SeverityInfo
			} else {
				results[i].Severity = domain.SeverityError
			}
		}
	}
}

// collectResults appends results to the report and updates AllPass.
func collectResults(report *domain.VerifyReport, results []domain.VerifyResult) {
	for _, r := range results {
		report.Results = append(report.Results, r)
		if !r.Passed {
			report.AllPass = false
		}
	}
}

// sortedMapKeys returns the keys of a string-keyed map in sorted order.
func sortedMapKeys(m map[string]domain.AdapterConfig) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
