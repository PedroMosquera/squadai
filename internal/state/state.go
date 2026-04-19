package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/PedroMosquera/squadai/internal/fileutil"
)

const (
	stateDir  = ".squadai"
	stateFile = "state.json"
)

// State holds persisted install state for SquadAI.
type State struct {
	InstalledAgents     []string  `json:"installed_agents"`
	LastApply           time.Time `json:"last_apply,omitempty"`
	LastUpdateCheck     time.Time `json:"last_update_check,omitempty"`
	UpdateChecksEnabled bool      `json:"update_checks_enabled,omitempty"`
}

// DefaultPath returns the canonical path to the state file: ~/.squadai/state.json.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, stateDir, stateFile), nil
}

// Load reads and decodes the state file at path.
// If the file does not exist, an empty State is returned without error.
func Load(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &State{InstalledAgents: []string{}}, nil
		}
		return nil, fmt.Errorf("read state file: %w", err)
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("decode state file: %w", err)
	}
	if s.InstalledAgents == nil {
		s.InstalledAgents = []string{}
	}
	return &s, nil
}

// Save encodes s and atomically writes it to path, creating parent directories
// as needed. InstalledAgents are sorted before serialization for deterministic output.
func Save(path string, s *State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	// Sort agents for deterministic JSON output.
	sorted := make([]string, len(s.InstalledAgents))
	copy(sorted, s.InstalledAgents)
	sort.Strings(sorted)
	out := &State{
		InstalledAgents:     sorted,
		LastApply:           s.LastApply,
		LastUpdateCheck:     s.LastUpdateCheck,
		UpdateChecksEnabled: s.UpdateChecksEnabled,
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("encode state: %w", err)
	}
	data = append(data, '\n')
	if _, err := fileutil.WriteAtomic(path, data, 0o644); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}
	return nil
}

// AddAgents merges ids into s.InstalledAgents: idempotent, sorted, deduped.
func (s *State) AddAgents(ids []string) {
	seen := make(map[string]bool, len(s.InstalledAgents)+len(ids))
	for _, id := range s.InstalledAgents {
		seen[id] = true
	}
	for _, id := range ids {
		if id != "" {
			seen[id] = true
		}
	}
	merged := make([]string, 0, len(seen))
	for id := range seen {
		merged = append(merged, id)
	}
	sort.Strings(merged)
	s.InstalledAgents = merged
}

// RemoveAgents removes the specified ids from s.InstalledAgents.
func (s *State) RemoveAgents(ids []string) {
	remove := make(map[string]bool, len(ids))
	for _, id := range ids {
		remove[id] = true
	}
	filtered := s.InstalledAgents[:0:0]
	for _, id := range s.InstalledAgents {
		if !remove[id] {
			filtered = append(filtered, id)
		}
	}
	if filtered == nil {
		filtered = []string{}
	}
	s.InstalledAgents = filtered
}
