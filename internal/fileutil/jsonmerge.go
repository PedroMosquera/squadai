package fileutil

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
)

// MergeConflict records a single key where the user-edited value on disk
// differs from SquadAI's incoming value AND SquadAI does not own the key.
// The caller reports these so the user can reconcile manually — MergeJSON
// never overwrites them.
type MergeConflict struct {
	Key           string
	UserValue     any
	IncomingValue any
}

// MergeJSON combines an existing on-disk JSON document with an incoming
// desired document, respecting user-wins semantics for any key SquadAI does
// not own. Ownership is tracked via `managed`, the list of keys SquadAI
// previously wrote for this file (read from the .squadai/managed.json sidecar
// by the caller).
//
// Per-key behavior:
//   - Absent from existing            → written; key added to newlyManaged.
//   - Present AND in managed          → overwritten; key added to newlyManaged.
//   - Present, not managed, identical → kept; key added to newlyManaged
//     (SquadAI now claims it, since existing == incoming).
//   - Present, not managed, different → existing kept; emitted as MergeConflict;
//     key NOT added to newlyManaged.
//
// Keys only in existing are always kept untouched.
//
// Nested objects are compared atomically — no deep merge. This matches how
// SquadAI's current call sites treat top-level keys (mcpServers entries,
// settings entries, permission entries) as opaque blobs.
//
// MergeJSON is pure: no IO. It returns a new map; the caller decides whether
// to write it via fileutil.WriteAtomic + managed.WriteManagedKeys.
//
// err is reserved for future use (deep-merge failure modes) and is currently
// always nil. It stays in the signature so callers can adopt richer semantics
// without another API break.
func MergeJSON(existing, incoming map[string]any, managed []string) (merged map[string]any, conflicts []MergeConflict, newlyManaged []string, err error) {
	// Both nil: nothing to merge, nothing to claim.
	if existing == nil && incoming == nil {
		return nil, nil, nil, nil
	}

	managedSet := make(map[string]bool, len(managed))
	for _, k := range managed {
		managedSet[k] = true
	}

	// Start from a shallow copy of existing so keys only in existing survive.
	merged = make(map[string]any, len(existing)+len(incoming))
	for k, v := range existing {
		merged[k] = v
	}

	// claimed dedupes newlyManaged (a key cannot be both written and conflict).
	claimed := make(map[string]bool, len(incoming))

	for k, incomingVal := range incoming {
		existingVal, present := existing[k]

		switch {
		case !present:
			merged[k] = incomingVal
			claimed[k] = true

		case managedSet[k]:
			merged[k] = incomingVal
			claimed[k] = true

		case reflect.DeepEqual(existingVal, incomingVal):
			claimed[k] = true

		default:
			conflicts = append(conflicts, MergeConflict{
				Key:           k,
				UserValue:     existingVal,
				IncomingValue: incomingVal,
			})
		}
	}

	if len(claimed) > 0 {
		newlyManaged = make([]string, 0, len(claimed))
		for k := range claimed {
			newlyManaged = append(newlyManaged, k)
		}
		sort.Strings(newlyManaged)
	}

	return merged, conflicts, newlyManaged, nil
}

// MergeAndWriteResult reports the outcome of MergeAndWriteJSON.
//
// When Conflicts is non-empty the file is NOT written, Written is false, and
// NewlyManaged is empty — the caller is expected to surface the conflicts
// (typically as domain.ConflictError) without persisting any change.
//
// When Conflicts is empty the merged document was written atomically, Written
// reports whether the content actually changed on disk (false means an
// identical file was already present), and NewlyManaged is the full set of
// top-level keys SquadAI claims for this file going forward.
type MergeAndWriteResult struct {
	Written      bool
	Created      bool
	Conflicts    []MergeConflict
	NewlyManaged []string
}

// MergeAndWriteJSON reads the JSON file at path, merges incoming with
// user-wins semantics, and writes the result atomically. overrides lists
// top-level keys the caller grants permission to overwrite even though
// SquadAI does not currently own them — typically sourced from an
// ApplyPolicy that carries the user's per-key consent from the review
// screen down to the installer.
//
// Output is marshaled with 2-space indentation and a trailing newline, which
// matches the style SquadAI uses elsewhere and minimizes churn when
// co-existing with user-maintained files. Installers with different style
// needs should use MergeJSON directly and write themselves.
//
// Returns an error only for IO / parse failures. Unresolved conflicts are
// reported as a VALUE in MergeAndWriteResult.Conflicts — the caller decides
// how to surface them (domain.ConflictError for CLI/TUI).
func MergeAndWriteJSON(path string, incoming map[string]any, managedKeys, overrides []string, perm os.FileMode) (MergeAndWriteResult, error) {
	existing, err := ReadJSONFile(path)
	if err != nil {
		return MergeAndWriteResult{}, fmt.Errorf("read existing: %w", err)
	}

	// Fold overrides into the managed set so MergeJSON treats them as
	// already-owned keys and overwrites without producing a conflict.
	// Dedup via a set so callers can pass either list without worry.
	effectiveManaged := managedKeys
	if len(overrides) > 0 {
		seen := make(map[string]bool, len(managedKeys)+len(overrides))
		effectiveManaged = make([]string, 0, len(managedKeys)+len(overrides))
		for _, k := range managedKeys {
			if !seen[k] {
				seen[k] = true
				effectiveManaged = append(effectiveManaged, k)
			}
		}
		for _, k := range overrides {
			if !seen[k] {
				seen[k] = true
				effectiveManaged = append(effectiveManaged, k)
			}
		}
	}

	merged, conflicts, newlyManaged, err := MergeJSON(existing, incoming, effectiveManaged)
	if err != nil {
		return MergeAndWriteResult{}, err
	}
	if len(conflicts) > 0 {
		return MergeAndWriteResult{Conflicts: conflicts}, nil
	}

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return MergeAndWriteResult{}, fmt.Errorf("marshal merged: %w", err)
	}
	data = append(data, '\n')

	wr, err := WriteAtomic(path, data, perm)
	if err != nil {
		return MergeAndWriteResult{}, fmt.Errorf("write %s: %w", path, err)
	}

	return MergeAndWriteResult{
		Written:      wr.Changed,
		Created:      wr.Created,
		NewlyManaged: newlyManaged,
	}, nil
}
