package state

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ─── Load ───────────────────────────────────────────────────────────────────

func TestLoad_MissingFile_ReturnsEmptyState(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "nonexistent", "state.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load on missing file should not error: %v", err)
	}
	if s == nil {
		t.Fatal("Load returned nil state")
	}
	if len(s.InstalledAgents) != 0 {
		t.Errorf("expected empty InstalledAgents, got %v", s.InstalledAgents)
	}
}

func TestLoad_InvalidJSON_ReturnsError(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")
	if err := os.WriteFile(path, []byte("not-json{{{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// ─── Save + Load roundtrip ───────────────────────────────────────────────────

func TestSaveLoad_Roundtrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, ".squadai", "state.json")

	ts := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	original := &State{
		InstalledAgents: []string{"claude", "cursor", "opencode"},
		LastApply:       ts,
	}

	if err := Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded.InstalledAgents) != len(original.InstalledAgents) {
		t.Fatalf("InstalledAgents length: want %d, got %d",
			len(original.InstalledAgents), len(loaded.InstalledAgents))
	}
	for i, id := range original.InstalledAgents {
		if loaded.InstalledAgents[i] != id {
			t.Errorf("InstalledAgents[%d]: want %q, got %q", i, id, loaded.InstalledAgents[i])
		}
	}
	if !loaded.LastApply.Equal(ts) {
		t.Errorf("LastApply: want %v, got %v", ts, loaded.LastApply)
	}
}

func TestSave_CreatesParentDirs(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "a", "b", "c", "state.json")

	s := &State{InstalledAgents: []string{"opencode"}}
	if err := Save(path, s); err != nil {
		t.Fatalf("Save should create parent dirs: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist after Save: %v", err)
	}
}

// ─── AddAgents ───────────────────────────────────────────────────────────────

func TestAddAgents_Idempotent(t *testing.T) {
	s := &State{InstalledAgents: []string{"opencode"}}
	s.AddAgents([]string{"opencode"})
	s.AddAgents([]string{"opencode"})
	if len(s.InstalledAgents) != 1 {
		t.Errorf("expected 1 agent after idempotent adds, got %d: %v", len(s.InstalledAgents), s.InstalledAgents)
	}
}

func TestAddAgents_SortedDeduped(t *testing.T) {
	tests := []struct {
		name     string
		initial  []string
		add      []string
		expected []string
	}{
		{
			name:     "merge and sort",
			initial:  []string{"opencode", "claude"},
			add:      []string{"cursor", "claude", "windsurf"},
			expected: []string{"claude", "cursor", "opencode", "windsurf"},
		},
		{
			name:     "empty initial",
			initial:  []string{},
			add:      []string{"vscode", "opencode"},
			expected: []string{"opencode", "vscode"},
		},
		{
			name:     "no new agents",
			initial:  []string{"claude", "opencode"},
			add:      []string{},
			expected: []string{"claude", "opencode"},
		},
		{
			name:     "skip empty strings",
			initial:  []string{},
			add:      []string{"", "opencode", ""},
			expected: []string{"opencode"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &State{InstalledAgents: tc.initial}
			s.AddAgents(tc.add)

			if len(s.InstalledAgents) != len(tc.expected) {
				t.Fatalf("want %v, got %v", tc.expected, s.InstalledAgents)
			}
			for i, id := range tc.expected {
				if s.InstalledAgents[i] != id {
					t.Errorf("[%d] want %q, got %q", i, id, s.InstalledAgents[i])
				}
			}
		})
	}
}

// ─── RemoveAgents ────────────────────────────────────────────────────────────

func TestRemoveAgents(t *testing.T) {
	tests := []struct {
		name     string
		initial  []string
		remove   []string
		expected []string
	}{
		{
			name:     "remove one",
			initial:  []string{"claude", "cursor", "opencode"},
			remove:   []string{"cursor"},
			expected: []string{"claude", "opencode"},
		},
		{
			name:     "remove all",
			initial:  []string{"claude", "opencode"},
			remove:   []string{"claude", "opencode"},
			expected: []string{},
		},
		{
			name:     "remove nonexistent is noop",
			initial:  []string{"opencode"},
			remove:   []string{"windsurf"},
			expected: []string{"opencode"},
		},
		{
			name:     "remove from empty",
			initial:  []string{},
			remove:   []string{"opencode"},
			expected: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &State{InstalledAgents: tc.initial}
			s.RemoveAgents(tc.remove)

			if len(s.InstalledAgents) != len(tc.expected) {
				t.Fatalf("want %v, got %v", tc.expected, s.InstalledAgents)
			}
			for i, id := range tc.expected {
				if s.InstalledAgents[i] != id {
					t.Errorf("[%d] want %q, got %q", i, id, s.InstalledAgents[i])
				}
			}
		})
	}
}

// ─── DefaultPath ─────────────────────────────────────────────────────────────

func TestDefaultPath_ExpandsHome(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}

	if !strings.HasPrefix(path, tmp) {
		t.Errorf("expected path under %q, got %q", tmp, path)
	}
	if !strings.HasSuffix(path, filepath.Join(".squadai", "state.json")) {
		t.Errorf("expected path ending in .squadai/state.json, got %q", path)
	}
}

// ─── UpdateChecks fields ──────────────────────────────────────────────────────

func TestSaveLoad_UpdateChecksFields(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	ts := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	original := &State{
		InstalledAgents:     []string{"claude"},
		LastUpdateCheck:     ts,
		UpdateChecksEnabled: true,
	}
	if err := Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !loaded.LastUpdateCheck.Equal(ts) {
		t.Errorf("LastUpdateCheck: want %v, got %v", ts, loaded.LastUpdateCheck)
	}
	if !loaded.UpdateChecksEnabled {
		t.Error("UpdateChecksEnabled should be true")
	}
}

func TestLoad_BackwardCompat_MissingUpdateFields(t *testing.T) {
	// Old state.json without update fields must load without error.
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")
	old := `{"installed_agents":["opencode"],"last_apply":"2026-01-01T00:00:00Z"}`
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.UpdateChecksEnabled {
		t.Error("UpdateChecksEnabled should default to false")
	}
	if !s.LastUpdateCheck.IsZero() {
		t.Error("LastUpdateCheck should default to zero")
	}
}

// ─── JSON output is deterministic ────────────────────────────────────────────

func TestSave_JSONDeterministic(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "state.json")

	s := &State{InstalledAgents: []string{"opencode", "claude", "cursor"}}
	if err := Save(path, s); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Agents should appear in sorted order.
	claudeIdx := strings.Index(content, "claude")
	cursorIdx := strings.Index(content, "cursor")
	opencodeIdx := strings.Index(content, "opencode")

	if claudeIdx > cursorIdx || cursorIdx > opencodeIdx {
		t.Errorf("agents not in sorted order in JSON:\n%s", content)
	}
}
