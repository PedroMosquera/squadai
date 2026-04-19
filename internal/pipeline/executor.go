package pipeline

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroMosquera/squadai/internal/backup"
	"github.com/PedroMosquera/squadai/internal/components/copilot"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/managed"
)

// Executor runs a plan and produces a step-level report.
type Executor struct {
	installers     map[domain.ComponentID]domain.ComponentInstaller
	copilotManager *copilot.Manager
	projectDir     string
	copilotCfg     domain.CopilotConfig
	backupStore    *backup.Store // nil = no backup/rollback
	sink           EventSink     // progress event sink; defaults to NopSink
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
		sink:           NopSink{},
	}
}

// WithSink sets the event sink for progress reporting.
// Call before Execute. Returns e to allow method chaining.
func (e *Executor) WithSink(s EventSink) *Executor {
	if s == nil {
		s = NopSink{}
	}
	e.sink = s
	return e
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

	total := len(plan)
	e.sink.Send(Event{Type: EventPipelineStart, Total: total})

	// Create backup if store is configured.
	if e.backupStore != nil {
		paths := collectTargetPaths(plan)
		if len(paths) > 0 {
			manifest, err := e.backupStore.SnapshotFiles(paths, "apply")
			if err != nil {
				e.sink.Send(Event{Type: EventPipelineDone, Total: total})
				return nil, fmt.Errorf("%w: %v", domain.ErrBackupFailed, err)
			}
			report.BackupID = manifest.ID
		}
	}

loop:
	for i, action := range plan {
		ev := Event{
			Component: string(action.Component),
			Adapter:   string(action.Agent),
			Action:    string(action.Action),
			Path:      action.TargetPath,
			Index:     i,
			Total:     total,
		}

		ev.Type = EventStepStart
		e.sink.Send(ev)

		result := e.executeOne(action)
		report.Steps = append(report.Steps, result)

		switch {
		case result.Status == domain.StepFailed:
			ev.Type = EventStepFailed
			if result.Error != "" {
				ev.Err = fmt.Errorf("%s", result.Error) //nolint:err113
			}
			e.sink.Send(ev)

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
						e.sink.Send(Event{Type: EventPipelineDone, Total: total})
						return report, fmt.Errorf("%w: %v", domain.ErrRollbackFailed, err)
					}
				}

				// Clean up managed sidecar entries for files tracked during
				// the failed apply sequence. The backup restore already removed
				// the newly created files from disk; we must keep the sidecar
				// consistent so the next apply sees a clean state.
				for _, step := range report.Steps[:i] {
					if step.Status == domain.StepSuccess &&
						step.Action.Action == domain.ActionCreate &&
						step.Action.TargetPath != "" {
						if relPath, relErr := filepath.Rel(e.projectDir, step.Action.TargetPath); relErr == nil {
							// Best-effort: ignore untrack errors.
							_ = managed.UntrackCreatedFile(e.projectDir, relPath)
						}
					}
				}

				break loop
			}
			// Without backup store, continue executing remaining actions (legacy).

		case action.Action == domain.ActionSkip:
			ev.Type = EventStepSkipped
			e.sink.Send(ev)

		default:
			ev.Type = EventStepDone
			e.sink.Send(ev)
		}
	}

	e.sink.Send(Event{Type: EventPipelineDone, Total: total})
	return report, nil
}

func (e *Executor) executeOne(action domain.PlannedAction) domain.StepResult {
	if action.Action == domain.ActionSkip {
		return domain.StepResult{
			Action: action,
			Status: domain.StepSuccess,
		}
	}

	// Handle delete actions: remove the target file if it exists.
	// Idempotent — non-existent files are treated as already deleted.
	if action.Action == domain.ActionDelete {
		if err := deleteFile(action.TargetPath); err != nil {
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

	// Track newly created files in the managed sidecar for reversibility.
	if action.Action == domain.ActionCreate && action.TargetPath != "" {
		if relPath, relErr := filepath.Rel(e.projectDir, action.TargetPath); relErr == nil {
			// Best-effort: ignore tracking errors so they don't fail the apply step.
			_ = managed.TrackCreatedFile(e.projectDir, relPath)
		}
	}

	return domain.StepResult{
		Action: action,
		Status: domain.StepSuccess,
	}
}

// deleteFile removes path from disk. It is idempotent: if the file does not
// exist, the call succeeds without error.
func deleteFile(path string) error {
	err := os.Remove(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
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
