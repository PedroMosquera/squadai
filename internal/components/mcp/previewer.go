package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/managed"
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

// detectConflicts returns the set of root-key conflicts between the existing
// on-disk file and what Apply would write. A conflict occurs when SquadAI's
// target root key ("mcp" or the adapter's MCPRootKey) is present on disk,
// not tracked in the managed-keys sidecar, and has a value different from
// what SquadAI would write.
func (i *Installer) detectConflicts(action domain.PlannedAction, projectDir string) ([]domain.Conflict, error) {
	existing, err := fileutil.ReadJSONFile(action.TargetPath)
	if err != nil {
		return nil, fmt.Errorf("read existing JSON: %w", err)
	}
	if existing == nil {
		return nil, nil
	}

	rootKey, incomingVal, err := i.rootKeyAndValue(action)
	if err != nil {
		return nil, err
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

	incoming := map[string]any{rootKey: incomingVal}
	_, merges, _, mergeErr := fileutil.MergeJSON(existing, incoming, managedKeys)
	if mergeErr != nil {
		return nil, mergeErr
	}
	if len(merges) == 0 {
		return nil, nil
	}

	out := make([]domain.Conflict, 0, len(merges))
	for _, c := range merges {
		out = append(out, domain.Conflict{
			Key:           c.Key,
			UserValue:     stringifyForConflict(c.UserValue),
			IncomingValue: stringifyForConflict(c.IncomingValue),
		})
	}
	return out, nil
}

// rootKeyAndValue returns the top-level JSON key SquadAI owns for this
// action's strategy, plus the value Apply would write under it.
func (i *Installer) rootKeyAndValue(action domain.PlannedAction) (string, any, error) {
	serversMap := make(map[string]any, len(i.servers))
	for name, def := range i.servers {
		serversMap[name] = serverToMap(def, action.Agent)
	}

	var rootKey string
	if isMCPConfigFileAction(action) {
		rootKey = i.rootKeyForAgent(action.Agent)
	} else {
		rootKey = mcpKey
	}

	// Round-trip through JSON so the value is the same concrete type that
	// MergeJSON will see on the existing side (map[string]any, etc.).
	normalized, err := normalizeJSON(serversMap)
	if err != nil {
		return "", nil, err
	}
	return rootKey, normalized, nil
}

// isMCPConfigFileAction checks the action description marker used to route
// between the two write strategies. Mirrors the check in Apply.
func isMCPConfigFileAction(action domain.PlannedAction) bool {
	const marker = "mcp:configfile:"
	return len(action.Description) >= len(marker) && action.Description[:len(marker)] == marker
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
