package memory

import (
	"fmt"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// Preview implements domain.Previewer for the memory installer. Each entry
// carries a unified diff of the marker section Apply would inject. No
// conflicts are surfaced: the marker block is a SquadAI-owned region inside
// a user file, so content outside the block stays untouched by design.
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

		existing, err := fileutil.ReadFileOrEmpty(action.TargetPath)
		if err != nil {
			return nil, fmt.Errorf("read existing %s: %w", action.TargetPath, err)
		}

		content := templateForAgentID(action.Agent)
		proposed := marker.InjectSection(string(existing), SectionID, content)

		entry.Diff = fileutil.UnifiedDiff(action.TargetPath, string(existing), proposed)
		entries = append(entries, entry)
	}

	return entries, nil
}
