// Package squadrefine holds the on-disk state and helpers for the
// /squadai-init slash command — a per-project record of when the agent
// last refined its sub-agent role files for this codebase.
//
// State lives at <projectDir>/.squadai/.squad-refined as JSON.
// squadai itself never *generates* refinements (that happens
// inside the agent at the user's invocation); this package only
// reads, validates, and updates the bookkeeping the agent writes
// back, plus computes the drift signals consumed by the
// session-start hook, the squadai status command, and the
// post-apply hint.
package squadrefine

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/PedroMosquera/squadai/internal/fileutil"
)

// FileName is the basename of the state file under .squadai/.
const FileName = ".squad-refined"

// CurrentVersion is the schema version this package writes.
const CurrentVersion = 1

// NudgeThrottleAt is the count of unactioned nudges after which the
// session-start hook stops emitting until refinement runs or a new
// drift signal fires.
const NudgeThrottleAt = 3

// State records the result of a /squadai-init run plus the drift
// state used by the session-start hook.
//
// SignalHashes records SHA-256 of each repo signal sampled at
// refinement time. Mismatch on recompute => drift.
//
// Files records SHA-256 of the refinement-block content per refined
// target file. Mismatch on recompute => user has hand-edited;
// drives the (k/m/o/a) prompt in /squadai-init.
//
// Nudges throttles the session-start hook so the user is not
// spammed across sessions.
type State struct {
	Version              int               `json:"version"`
	LastRunAt            string            `json:"last_run_at"`
	MethodologyAtLastRun string            `json:"methodology_at_last_run,omitempty"`
	SignalHashes         map[string]string `json:"signal_hashes,omitempty"`
	Files                map[string]string `json:"files,omitempty"`
	Nudges               NudgeState        `json:"nudges"`
}

// NudgeState carries hook self-throttle state.
type NudgeState struct {
	UnactionedCount int    `json:"unactioned_count"`
	Throttled       bool   `json:"throttled"`
	LastSignature   string `json:"last_signature,omitempty"`
}

// FilePath returns the full path to the state file under projectDir.
func FilePath(projectDir string) string {
	return filepath.Join(projectDir, ".squadai", FileName)
}

// Load reads the state file from projectDir.
//
// Returns:
//   - (state, true, nil) when the file exists and parsed cleanly.
//   - (nil, false, nil) when the file does not exist.
//   - (nil, false, err) when the file exists but is unreadable or
//     parses to an incompatible version.
func Load(projectDir string) (*State, bool, error) {
	path := FilePath(projectDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("read %s: %w", path, err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, false, fmt.Errorf("parse %s: %w", path, err)
	}

	if s.Version > CurrentVersion {
		return nil, false, fmt.Errorf(
			"%s: schema version %d is newer than this squadai supports (max %d)",
			path, s.Version, CurrentVersion,
		)
	}
	if s.Version == 0 {
		// Pre-versioned files: coerce to current schema.
		s.Version = CurrentVersion
	}

	if s.SignalHashes == nil {
		s.SignalHashes = map[string]string{}
	}
	if s.Files == nil {
		s.Files = map[string]string{}
	}

	return &s, true, nil
}

// Save writes the state file under projectDir atomically.
//
// Output is canonicalized — Go's json.Marshal sorts map keys, so
// the file is byte-stable across runs (helpful for `git diff`).
func Save(projectDir string, s *State) error {
	if s == nil {
		return errors.New("squadrefine.Save: nil state")
	}
	if s.Version == 0 {
		s.Version = CurrentVersion
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal squad-refined: %w", err)
	}
	data = append(data, '\n')

	path := FilePath(projectDir)
	if _, err := fileutil.WriteAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// HashContent returns the SHA-256 of content as a "sha256:<hex>" string.
// Used for both signal hashes and file marker-block hashes so they
// share one representation.
func HashContent(content []byte) string {
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// IsFresh reports whether the on-disk state matches the current
// signal hashes — i.e., no drift since the last /squadai-init run.
//
// Returns false when state is nil or has no recorded signals.
func IsFresh(s *State, currentSignals map[string]string) bool {
	if s == nil || len(s.SignalHashes) == 0 {
		return false
	}
	for k, recorded := range s.SignalHashes {
		if currentSignals[k] != recorded {
			return false
		}
	}
	for k := range currentSignals {
		if _, ok := s.SignalHashes[k]; !ok {
			return false
		}
	}
	return true
}

// DriftReasons returns the names of signals that differ between the
// recorded state and the current snapshot. Empty slice means fresh.
//
// Returns ["never-refined"] when state is nil or has no recorded
// signals — special-cased to give callers a single self-explanatory
// reason without leaking implementation detail.
func DriftReasons(s *State, currentSignals map[string]string) []string {
	if s == nil || len(s.SignalHashes) == 0 {
		return []string{"never-refined"}
	}
	var reasons []string
	for k, recorded := range s.SignalHashes {
		if currentSignals[k] != recorded {
			reasons = append(reasons, k)
		}
	}
	for k := range currentSignals {
		if _, ok := s.SignalHashes[k]; !ok {
			reasons = append(reasons, k+":new")
		}
	}
	sort.Strings(reasons)
	return reasons
}

// NoteNudgeFired increments the unactioned-nudge counter. When the
// new signature differs from LastSignature, the counter resets first
// (so a *different* drift signal gets fresh nudges even after the
// previous one was throttled). Caller is responsible for Save.
func NoteNudgeFired(s *State, signature string) NudgeState {
	if s == nil {
		return NudgeState{}
	}
	if s.Nudges.LastSignature != signature {
		s.Nudges.UnactionedCount = 0
		s.Nudges.Throttled = false
		s.Nudges.LastSignature = signature
	}
	s.Nudges.UnactionedCount++
	if s.Nudges.UnactionedCount >= NudgeThrottleAt {
		s.Nudges.Throttled = true
	}
	return s.Nudges
}

// ResetNudges clears throttle state. Called after a successful
// /squadai-init run records new file hashes.
func ResetNudges(s *State) {
	if s == nil {
		return
	}
	s.Nudges = NudgeState{}
}
