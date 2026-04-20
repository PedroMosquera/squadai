package settings

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/managed"
)

// Preview implements domain.Previewer for the settings installer. It returns
// one entry per planned action with a unified diff plus any top-level key
// conflicts that would block a clean apply. Pure: no file mutation.
func (i *Installer) Preview(adapter domain.Adapter, homeDir, projectDir string) ([]domain.PreviewEntry, error) {
	actions, err := i.Plan(adapter, homeDir, projectDir)
	if err != nil {
		return nil, err
	}

	entries := make([]domain.PreviewEntry, 0, len(actions))
	for _, action := range actions {
		entry := domain.PreviewEntry{
			Component:  action.Component,
			Action:     action.Action,
			TargetPath: action.TargetPath,
		}

		if action.Action == domain.ActionSkip {
			entries = append(entries, entry)
			continue
		}

		existingBytes, err := readFileOrEmpty(action.TargetPath)
		if err != nil {
			return nil, fmt.Errorf("read existing %s: %w", action.TargetPath, err)
		}

		proposedBytes, err := i.RenderContent(action)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", action.TargetPath, err)
		}

		entry.Diff = fileutil.UnifiedDiff(action.TargetPath, string(existingBytes), string(proposedBytes))

		if action.Action == domain.ActionUpdate {
			conflicts, err := i.detectConflicts(action, projectDir)
			if err != nil {
				return nil, err
			}
			entry.Conflicts = conflicts
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

// detectConflicts returns the set of top-level key conflicts between the
// existing on-disk file and what Apply would write — i.e. keys SquadAI
// manages, present on disk, with a value the user has hand-edited.
func (i *Installer) detectConflicts(action domain.PlannedAction, projectDir string) ([]domain.Conflict, error) {
	existing, err := fileutil.ReadJSONFile(action.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("read existing JSON: %w", err)
	}
	if existing == nil {
		return nil, nil
	}

	agentID := string(action.Agent)
	settings := i.adapterSettings[agentID]
	if len(settings) == 0 {
		return nil, nil
	}

	incoming := make(map[string]any, len(settings)+1)
	for k, v := range settings {
		incoming[k] = v
	}
	if agentID == string(domain.AgentOpenCode) {
		incoming["$schema"] = "https://opencode.ai/config.json"
	}

	relPath := action.TargetPath
	if projectDir != "" {
		if rel, relErr := filepath.Rel(projectDir, action.TargetPath); relErr == nil {
			relPath = rel
		}
	}
	managedKeys, err := managed.ReadManagedKeys(projectDir, relPath)
	if err != nil {
		return nil, fmt.Errorf("read managed keys: %w", err)
	}

	_, merges, _, mergeErr := fileutil.MergeJSON(existing, incoming, managedKeys)
	if mergeErr != nil {
		return nil, mergeErr
	}
	if len(merges) == 0 {
		return nil, nil
	}
	return conflictsToDomain(merges), nil
}

// readFileOrEmpty reads a file, returning nil if it does not exist.
func readFileOrEmpty(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return data, nil
}
