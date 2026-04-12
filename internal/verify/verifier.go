package verify

import (
	"github.com/PedroMosquera/agent-manager-pro/internal/components/copilot"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/memory"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/rules"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
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
		rulesInstaller = rules.New(cfg.Rules, projectDir)
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
			for _, r := range results {
				report.Results = append(report.Results, r)
				if !r.Passed {
					report.AllPass = false
				}
			}
		}

		// Rules component.
		if rulesInstaller != nil {
			results, err := rulesInstaller.Verify(adapter, homeDir, projectDir)
			if err != nil {
				return nil, err
			}
			for _, r := range results {
				report.Results = append(report.Results, r)
				if !r.Passed {
					report.AllPass = false
				}
			}
		}
	}

	// Verify copilot instructions.
	if cfg.Copilot.InstructionsTemplate != "" {
		results := v.copilotManager.Verify(projectDir, cfg.Copilot)
		for _, r := range results {
			report.Results = append(report.Results, r)
			if !r.Passed {
				report.AllPass = false
			}
		}
	}

	// Report policy violations as warnings.
	if len(cfg.Violations) > 0 {
		for _, violation := range cfg.Violations {
			report.Results = append(report.Results, domain.VerifyResult{
				Check:   "policy-override",
				Passed:  true, // violations are informational — policy value won
				Message: violation,
			})
		}
	}

	return report, nil
}
