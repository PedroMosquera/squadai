package verify

import (
	"context"
	"fmt"
	"sort"

	"github.com/PedroMosquera/squadai/internal/components/bundle"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// Verifier runs post-apply compliance checks across all components and adapters.
// Construction is intentionally empty: component installers are built via the
// shared bundle on each Verify call (or supplied by the caller via VerifyWithSet)
// so the planner and verifier cannot drift in how they instantiate components.
type Verifier struct{}

// New returns a Verifier.
func New() *Verifier {
	return &Verifier{}
}

// Verify runs all checks and produces a report. Installers are built via the
// shared bundle package — the same construction path the planner uses.
func (v *Verifier) Verify(cfg *domain.MergedConfig, adapters []domain.Adapter, homeDir, projectDir string) (*domain.VerifyReport, error) {
	set, err := bundle.Build(cfg, projectDir, bundle.Options{})
	if err != nil {
		return nil, err
	}
	return v.VerifyWithSet(set, cfg, adapters, homeDir, projectDir)
}

// VerifyWithSet runs verification against a caller-supplied installer Set.
// Use this when you already have a Set built for another purpose (e.g., the
// planner's) and want to guarantee the verifier sees the exact same installer
// instances — including any state attached after construction.
func (v *Verifier) VerifyWithSet(set *bundle.Set, cfg *domain.MergedConfig, adapters []domain.Adapter, homeDir, projectDir string) (*domain.VerifyReport, error) {
	report := &domain.VerifyReport{
		AllPass: true,
	}

	// Verify components for each enabled adapter.
	for _, adapter := range adapters {
		adapterCfg, ok := cfg.Adapters[string(adapter.ID())]
		if !ok || !adapterCfg.Enabled {
			continue
		}

		// Memory component.
		if memCfg, ok := cfg.Components[string(domain.ComponentMemory)]; ok && memCfg.Enabled && set.Memory != nil {
			results, err := set.Memory.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "memory")
			collectResults(report, results)
		}

		// Rules component.
		if set.Rules != nil {
			results, err := set.Rules.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "rules")
			collectResults(report, results)
		}

		// Settings component.
		if set.Settings != nil {
			results, err := set.Settings.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "settings")
			collectResults(report, results)
		}

		// MCP component.
		if set.MCP != nil {
			results, err := set.MCP.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "mcp")
			collectResults(report, results)
		}

		// Agents component.
		if set.Agents != nil {
			results, err := set.Agents.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "agents")
			collectResults(report, results)
		}

		// Skills component.
		if set.Skills != nil {
			results, err := set.Skills.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "skills")
			collectResults(report, results)
		}

		// Commands component.
		if set.Commands != nil {
			results, err := set.Commands.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "commands")
			collectResults(report, results)
		}

		// Plugins component.
		if set.Plugins != nil {
			results, err := set.Plugins.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "plugins")
			collectResults(report, results)
		}

		// Workflows component.
		if set.Workflows != nil {
			results, err := set.Workflows.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			tagResults(results, "workflows")
			collectResults(report, results)
		}
	}

	// Verify copilot instructions.
	if cfg.Copilot.InstructionsTemplate != "" && set.Copilot != nil {
		results := set.Copilot.Verify(projectDir, cfg.Copilot)
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
