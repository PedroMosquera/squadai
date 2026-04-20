package fileutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMergeJSON(t *testing.T) {
	cases := []struct {
		name             string
		existing         map[string]any
		incoming         map[string]any
		managed          []string
		wantMerged       map[string]any
		wantConflicts    []MergeConflict
		wantNewlyManaged []string
	}{
		{
			name:             "fresh_install_no_existing",
			existing:         nil,
			incoming:         map[string]any{"a": float64(1), "b": float64(2)},
			managed:          nil,
			wantMerged:       map[string]any{"a": float64(1), "b": float64(2)},
			wantConflicts:    nil,
			wantNewlyManaged: []string{"a", "b"},
		},
		{
			name:       "user_wins_conflict",
			existing:   map[string]any{"a": "user"},
			incoming:   map[string]any{"a": "us"},
			managed:    nil,
			wantMerged: map[string]any{"a": "user"},
			wantConflicts: []MergeConflict{
				{Key: "a", UserValue: "user", IncomingValue: "us"},
			},
			wantNewlyManaged: nil,
		},
		{
			name:             "safe_overwrite_of_managed_key",
			existing:         map[string]any{"a": "old"},
			incoming:         map[string]any{"a": "new"},
			managed:          []string{"a"},
			wantMerged:       map[string]any{"a": "new"},
			wantConflicts:    nil,
			wantNewlyManaged: []string{"a"},
		},
		{
			name:             "user_added_unchanged_key_is_claimed",
			existing:         map[string]any{"a": "v"},
			incoming:         map[string]any{"a": "v"},
			managed:          nil,
			wantMerged:       map[string]any{"a": "v"},
			wantConflicts:    nil,
			wantNewlyManaged: []string{"a"},
		},
		{
			name:             "empty_incoming_preserves_existing",
			existing:         map[string]any{"a": float64(1)},
			incoming:         map[string]any{},
			managed:          []string{"a"},
			wantMerged:       map[string]any{"a": float64(1)},
			wantConflicts:    nil,
			wantNewlyManaged: nil,
		},
		{
			name:       "nested_object_atomic_comparison",
			existing:   map[string]any{"s": map[string]any{"x": float64(1)}},
			incoming:   map[string]any{"s": map[string]any{"x": float64(2)}},
			managed:    nil,
			wantMerged: map[string]any{"s": map[string]any{"x": float64(1)}},
			wantConflicts: []MergeConflict{
				{
					Key:           "s",
					UserValue:     map[string]any{"x": float64(1)},
					IncomingValue: map[string]any{"x": float64(2)},
				},
			},
			wantNewlyManaged: nil,
		},
		{
			name:             "existing_keys_not_in_incoming_preserved",
			existing:         map[string]any{"a": float64(1), "b": float64(2)},
			incoming:         map[string]any{"c": float64(3)},
			managed:          nil,
			wantMerged:       map[string]any{"a": float64(1), "b": float64(2), "c": float64(3)},
			wantConflicts:    nil,
			wantNewlyManaged: []string{"c"},
		},
		{
			name:             "both_nil_returns_nil",
			existing:         nil,
			incoming:         nil,
			managed:          nil,
			wantMerged:       nil,
			wantConflicts:    nil,
			wantNewlyManaged: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			merged, conflicts, newlyManaged, err := MergeJSON(tc.existing, tc.incoming, tc.managed)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !reflect.DeepEqual(merged, tc.wantMerged) {
				t.Errorf("merged = %#v, want %#v", merged, tc.wantMerged)
			}
			if !reflect.DeepEqual(conflicts, tc.wantConflicts) {
				t.Errorf("conflicts = %#v, want %#v", conflicts, tc.wantConflicts)
			}
			if !reflect.DeepEqual(newlyManaged, tc.wantNewlyManaged) {
				t.Errorf("newlyManaged = %#v, want %#v", newlyManaged, tc.wantNewlyManaged)
			}
		})
	}
}

func TestMergeAndWriteJSON_FreshInstall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	incoming := map[string]any{"mcp": map[string]any{"s": map[string]any{"url": "x"}}}
	res, err := MergeAndWriteJSON(path, incoming, nil, nil, 0644)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !res.Written || !res.Created {
		t.Errorf("Written=%v Created=%v, want both true", res.Written, res.Created)
	}
	if len(res.Conflicts) != 0 {
		t.Errorf("Conflicts = %v, want none", res.Conflicts)
	}
	if !reflect.DeepEqual(res.NewlyManaged, []string{"mcp"}) {
		t.Errorf("NewlyManaged = %v, want [mcp]", res.NewlyManaged)
	}

	// File should round-trip cleanly.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("parse back: %v", err)
	}
	if !reflect.DeepEqual(got, incoming) {
		t.Errorf("round-trip mismatch: %v vs %v", got, incoming)
	}
}

func TestMergeAndWriteJSON_ManagedKeyOverwrites(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Pre-seed a stale SquadAI-owned value.
	seed := map[string]any{"mcp": map[string]any{"s": map[string]any{"url": "old"}}}
	seedData, _ := json.Marshal(seed)
	if err := os.WriteFile(path, seedData, 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	incoming := map[string]any{"mcp": map[string]any{"s": map[string]any{"url": "new"}}}
	res, err := MergeAndWriteJSON(path, incoming, []string{"mcp"}, nil, 0644)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(res.Conflicts) != 0 {
		t.Fatalf("Conflicts = %v, want none", res.Conflicts)
	}
	if !res.Written {
		t.Error("expected write")
	}

	data, _ := os.ReadFile(path)
	var got map[string]any
	_ = json.Unmarshal(data, &got)
	want := map[string]any{"mcp": map[string]any{"s": map[string]any{"url": "new"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("file = %v, want %v", got, want)
	}
}

func TestMergeAndWriteJSON_UnmanagedKeyConflictsNoWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	user := map[string]any{"mcp": map[string]any{"user": map[string]any{"url": "user"}}}
	seedData, _ := json.Marshal(user)
	if err := os.WriteFile(path, seedData, 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	incoming := map[string]any{"mcp": map[string]any{"sq": map[string]any{"url": "sq"}}}
	res, err := MergeAndWriteJSON(path, incoming, nil, nil, 0644)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(res.Conflicts) != 1 || res.Conflicts[0].Key != "mcp" {
		t.Fatalf("Conflicts = %v, want single conflict on 'mcp'", res.Conflicts)
	}
	if res.Written {
		t.Error("file should not have been written")
	}

	// File on disk must be untouched.
	data, _ := os.ReadFile(path)
	var got map[string]any
	_ = json.Unmarshal(data, &got)
	if !reflect.DeepEqual(got, user) {
		t.Errorf("file mutated: got %v, want %v", got, user)
	}
}

func TestMergeAndWriteJSON_OverrideOverwritesUnmanagedKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	user := map[string]any{"mcp": map[string]any{"user": map[string]any{"url": "user"}}}
	seedData, _ := json.Marshal(user)
	if err := os.WriteFile(path, seedData, 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	incoming := map[string]any{"mcp": map[string]any{"sq": map[string]any{"url": "sq"}}}
	res, err := MergeAndWriteJSON(path, incoming, nil, []string{"mcp"}, 0644)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(res.Conflicts) != 0 {
		t.Fatalf("Conflicts = %v, want none (override granted)", res.Conflicts)
	}
	if !res.Written {
		t.Error("expected write")
	}
	if !reflect.DeepEqual(res.NewlyManaged, []string{"mcp"}) {
		t.Errorf("NewlyManaged = %v, want [mcp]", res.NewlyManaged)
	}

	data, _ := os.ReadFile(path)
	var got map[string]any
	_ = json.Unmarshal(data, &got)
	want := map[string]any{"mcp": map[string]any{"sq": map[string]any{"url": "sq"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("file = %v, want %v", got, want)
	}
}

func TestMergeAndWriteJSON_IdempotentOnIdenticalContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	incoming := map[string]any{"mcp": map[string]any{"s": "v"}}

	// First write creates.
	if _, err := MergeAndWriteJSON(path, incoming, nil, nil, 0644); err != nil {
		t.Fatalf("first: %v", err)
	}

	// Second write should be a no-op (no change).
	res, err := MergeAndWriteJSON(path, incoming, []string{"mcp"}, nil, 0644)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if res.Written {
		t.Error("expected no write on identical content")
	}
	if res.Created {
		t.Error("Created should be false on second call")
	}
}
