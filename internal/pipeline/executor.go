package pipeline

import (
	"fmt"

	"github.com/PedroMosquera/agent-manager-pro/internal/backup"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/copilot"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// Executor runs a plan and produces a step-level report.
type Executor struct {
	installers     map[domain.ComponentID]domain.ComponentInstaller
	copilotManager *copilot.Manager
	projectDir     string
	copilotCfg     domain.CopilotConfig
	backupStore    *backup.Store // nil = no backup/rollback
}

// New returns an Executor configured with component installers and copilot manager.
// If backupStore is nil, the executor runs without backup/rollback (legacy behavior).
func New(
	installers map[domain.ComponentID]domain.ComponentInstaller,
	copilotMgr *copilot.Manager,
	projectDir string,
	copilotCfg domain.CopilotConfig,
	backupStore *backup.Store,
) *Executor {
	return &Executor{
		installers:     installers,
		copilotManager: copilotMgr,
		projectDir:     projectDir,
		copilotCfg:     copilotCfg,
		backupStore:    backupStore,
	}
}

// Execute runs all planned actions in order, collecting step results.
//
// When a backup store is configured:
//   - All target files are snapshotted before any mutations.
//   - On first failure, remaining actions are marked as rolled_back and
//     all files are restored from the snapshot.
//   - Returns an error only if backup creation or rollback itself fails.
//
// When no backup store is configured (legacy behavior):
//   - All actions are attempted regardless of failures.
//   - Individual failures are recorded in the report.
func (e *Executor) Execute(plan []domain.PlannedAction) (*domain.ApplyReport, error) {
	report := &domain.ApplyReport{
		Steps:   make([]domain.StepResult, 0, len(plan)),
		Success: true,
	}

	// Create backup if store is configured.
	if e.backupStore != nil {
		paths := collectTargetPaths(plan)
		if len(paths) > 0 {
			manifest, err := e.backupStore.SnapshotFiles(paths, "apply")
			if err != nil {
				return nil, fmt.Errorf("%w: %v", domain.ErrBackupFailed, err)
			}
			report.BackupID = manifest.ID
		}
	}

	for i, action := range plan {
		result := e.executeOne(action)
		report.Steps = append(report.Steps, result)

		if result.Status == domain.StepFailed {
			report.Success = false

			if e.backupStore != nil {
				// Mark remaining actions as rolled back.
				for _, remaining := range plan[i+1:] {
					report.Steps = append(report.Steps, domain.StepResult{
						Action: remaining,
						Status: domain.StepRolledBack,
					})
				}

				// Rollback from backup.
				if report.BackupID != "" {
					if err := e.backupStore.Rollback(report.BackupID); err != nil {
						return report, fmt.Errorf("%w: %v", domain.ErrRollbackFailed, err)
					}
				}
				break
			}
			// Without backup store, continue executing remaining actions (legacy).
		}
	}

	return report, nil
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
		err = e.copilotManager.Apply(e.projectDir, e.copilotCfg)
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

// collectTargetPaths extracts unique target paths from non-skip actions.
func collectTargetPaths(plan []domain.PlannedAction) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, action := range plan {
		if action.Action == domain.ActionSkip {
			continue
		}
		if action.TargetPath != "" && !seen[action.TargetPath] {
			seen[action.TargetPath] = true
			paths = append(paths, action.TargetPath)
		}
	}
	return paths
}
