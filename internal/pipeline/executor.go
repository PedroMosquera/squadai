package pipeline

import (
	"fmt"

	"github.com/alexmosquera/agent-manager-pro/internal/components/copilot"
	"github.com/alexmosquera/agent-manager-pro/internal/domain"
)

// Executor runs a plan and produces a step-level report.
type Executor struct {
	installers     map[domain.ComponentID]domain.ComponentInstaller
	copilotManager *copilot.Manager
	projectDir     string
	copilotTmpl    string
}

// New returns an Executor configured with component installers and copilot manager.
func New(
	installers map[domain.ComponentID]domain.ComponentInstaller,
	copilotMgr *copilot.Manager,
	projectDir string,
	copilotTemplate string,
) *Executor {
	return &Executor{
		installers:     installers,
		copilotManager: copilotMgr,
		projectDir:     projectDir,
		copilotTmpl:    copilotTemplate,
	}
}

// Execute runs all planned actions in order, collecting step results.
// It does not stop on first failure — all actions are attempted and
// individual failures are recorded in the report.
func (e *Executor) Execute(plan []domain.PlannedAction) *domain.ApplyReport {
	report := &domain.ApplyReport{
		Steps:   make([]domain.StepResult, 0, len(plan)),
		Success: true,
	}

	for _, action := range plan {
		result := e.executeOne(action)
		report.Steps = append(report.Steps, result)
		if result.Status == domain.StepFailed {
			report.Success = false
		}
	}

	return report
}

func (e *Executor) executeOne(action domain.PlannedAction) domain.StepResult {
	if action.Action == domain.ActionSkip {
		return domain.StepResult{
			Action: action,
			Status: domain.StepSuccess,
		}
	}

	var err error

	// Route to the right handler.
	if action.ID == "copilot-instructions" {
		err = e.copilotManager.Apply(e.projectDir, e.copilotTmpl)
	} else if action.Component != "" {
		installer, ok := e.installers[action.Component]
		if !ok {
			return domain.StepResult{
				Action: action,
				Status: domain.StepFailed,
				Error:  fmt.Sprintf("no installer for component %q", action.Component),
			}
		}
		err = installer.Apply(action)
	} else {
		return domain.StepResult{
			Action: action,
			Status: domain.StepFailed,
			Error:  "action has no component and is not copilot-instructions",
		}
	}

	if err != nil {
		return domain.StepResult{
			Action: action,
			Status: domain.StepFailed,
			Error:  err.Error(),
		}
	}

	return domain.StepResult{
		Action: action,
		Status: domain.StepSuccess,
	}
}
