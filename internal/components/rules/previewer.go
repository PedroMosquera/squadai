package rules

import (
	"fmt"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// Preview implements domain.Previewer for the rules installer. Each entry
// carries a unified diff of the team-standards change Apply would make.
//
// Two shapes are covered:
//   - Marker-based (AGENTS.md, CLAUDE.md): the diff reflects the marker
//     section injection; content outside the block stays untouched.
//   - Structured rules (Windsurf, Cursor) with YAML frontmatter: the file
//     is full-file-owned by SquadAI and diff is against the full body.
//
// No conflicts are surfaced: both branches have well-defined ownership.
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

		var proposed string
		if fm := i.agentFrontmatter[action.Agent]; fm != "" {
			proposed = fm + i.content
			if !strings.HasSuffix(proposed, "\n") {
				proposed += "\n"
			}
		} else {
			proposed = marker.InjectSection(string(existing), SectionID, i.content)
		}

		entry.Diff = fileutil.UnifiedDiff(action.TargetPath, string(existing), proposed)
		entries = append(entries, entry)
	}

	return entries, nil
}
