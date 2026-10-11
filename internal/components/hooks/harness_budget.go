package hooks

import "github.com/PedroMosquera/squadai/internal/domain"

// WithUsage sets the usage config that drives budget enforcement in Codex
// (.codex/hooks.json) and Pi (a project extension). Claude budget hooks come
// through New's desired set instead, see WithBudgetHooks.
func (i *Installer) WithUsage(usage domain.UsageConfig) *Installer {
	i.usage = usage
	return i
}

func budgetHarness(agent domain.AgentID) bool {
	return agent == domain.AgentCodex || agent == domain.AgentPi
}

func (i *Installer) planHarnessBudget(adapter domain.Adapter, projectDir string) ([]domain.PlannedAction, error) {
	if adapter.ID() == domain.AgentCodex {
		return i.planCodexBudget(projectDir)
	}
	return i.planPiBudget(projectDir)
}

func (i *Installer) applyHarnessBudget(action domain.PlannedAction) error {
	if action.Agent == domain.AgentCodex {
		return i.applyCodexBudget(action)
	}
	return i.applyPiBudget(action)
}

func (i *Installer) verifyHarnessBudget(adapter domain.Adapter, projectDir string) []domain.VerifyResult {
	if adapter.ID() == domain.AgentCodex {
		return i.verifyCodexBudget(projectDir)
	}
	return i.verifyPiBudget(projectDir)
}

func (i *Installer) renderHarnessBudget(action domain.PlannedAction) ([]byte, error) {
	if action.Agent == domain.AgentCodex {
		out, _, err := i.renderCodexHooks(action.TargetPath)
		return out, err
	}
	return []byte(PiBudgetExtension(i.usage)), nil
}
