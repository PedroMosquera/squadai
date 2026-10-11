package plugins

import (
	"fmt"
	"os"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
)

// Preview implements domain.Previewer for the plugins installer. It returns
// one entry per planned action with a unified diff. The Claude settings
// entry also lists declared plugins or marketplaces the user already set to a
// different value, which Apply refuses to overwrite.
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

		if action.Action == domain.ActionSkip || action.TargetPath == "" {
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
		if action.Description == claudeSettingsDescription {
			m, err := i.mergeClaude(action.TargetPath, projectDir)
			if err != nil {
				return nil, err
			}
			if len(m.conflicts) > 0 {
				entry.Conflicts = conflictsToDomain(m.conflicts)
			}
		}
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
