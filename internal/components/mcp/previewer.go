package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
)

// Preview implements domain.Previewer. It returns one entry per planned
// action with a unified diff of the file delta plus any user-wins conflicts
// that would block a clean overwrite. It is read-only: no file is written,
// no sidecar is touched.
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

// detectConflicts returns the servers Apply would refuse to overwrite: a
// server SquadAI configures that is already on disk under the same name, not
// owned by SquadAI, and different from what SquadAI would write.
func (i *Installer) detectConflicts(action domain.PlannedAction, projectDir string) ([]domain.Conflict, error) {
	// TOML targets (Codex's config.toml) are marker-managed: Apply rewrites
	// only the hash-marker block and preserves user TOML outside it verbatim,
	// so root-key conflicts cannot occur — and the file must never be parsed
	// as JSON.
	if strings.HasPrefix(action.Description, "mcp:toml:") {
		return nil, nil
	}
	// The legacy migration only removes servers squadai owns; it never writes
	// over a user value.
	if strings.HasPrefix(action.Description, legacyPrefix) {
		return nil, nil
	}
	m, err := i.mergeServers(action.Agent, action.TargetPath, projectDir, nil)
	if err != nil {
		return nil, err
	}
	if len(m.conflicts) == 0 {
		return nil, nil
	}
	return conflictsToDomain(m.conflicts), nil
}

// normalizeJSON round-trips v through encoding/json so values end up as the
// generic types MergeJSON's reflect.DeepEqual expects
// (map[string]interface{}, []interface{}, float64, string, bool, nil).
func normalizeJSON(v any) (any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal for compare: %w", err)
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("unmarshal for compare: %w", err)
	}
	return out, nil
}

// readFileOrEmpty returns the file's bytes or an empty slice if the file
// does not exist. Non-ENOENT errors are propagated.
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

// stringifyForConflict renders an arbitrary JSON value as a compact, truncated
// string safe for TUI display.
func stringifyForConflict(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	const maxLen = 80
	s := string(data)
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}
