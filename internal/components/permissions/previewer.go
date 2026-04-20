package permissions

import (
	"fmt"
	"os"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
)

// Preview implements domain.Previewer for the permissions installer. It
// returns one entry per planned action with a unified diff.
//
// Permissions does not surface top-level key conflicts here: its Apply is
// already user-preserving (Claude's `permissions.deny` / `permissions.ask`
// arrays are appended to, not overwritten; OpenCode's `permission` map is
// deep-merged). The diff is enough for the review screen — the user can
// see exactly what Apply would change.
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
		entries = append(entries, entry)
	}

	return entries, nil
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
