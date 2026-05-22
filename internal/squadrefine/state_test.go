package squadrefine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func setupTempProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squadai"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return dir
}

func TestLoad_FileMissingReturnsFalseNoError(t *testing.T) {
	dir := setupTempProject(t)
	state, exists, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if exists {
		t.Fatalf("exists=true on empty project")
	}
	if state != nil {
		t.Fatalf("state should be nil when file missing")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := setupTempProject(t)
	in := &State{
		LastRunAt:            "2026-04-29T14:32:00Z",
		MethodologyAtLastRun: "tdd",
		SignalHashes: map[string]string{
			"go.mod":           "sha256:abc",
			"top_level_layout": "sha256:def",
		},
		Files: map[string]string{
			".claude/agents/orchestrator.md": "sha256:111",
		},
		Nudges: NudgeState{
			UnactionedCount: 1,
			LastSignature:   "sig-1",
		},
	}
	if err := Save(dir, in); err != nil {
		t.Fatalf("Save: %v", err)
	}

	out, exists, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !exists {
		t.Fatalf("Load.exists=false after Save")
	}
	if out.Version != CurrentVersion {
		t.Fatalf("Version=%d want %d", out.Version, CurrentVersion)
	}
	if !reflect.DeepEqual(out.SignalHashes, in.SignalHashes) {
		t.Errorf("SignalHashes mismatch: got %v want %v", out.SignalHashes, in.SignalHashes)
	}
	if !reflect.DeepEqual(out.Files, in.Files) {
		t.Errorf("Files mismatch: got %v want %v", out.Files, in.Files)
	}
	if out.Nudges != in.Nudges {
		t.Errorf("Nudges mismatch: got %+v want %+v", out.Nudges, in.Nudges)
	}
}

func TestSave_OutputIsStable(t *testing.T) {
	dir := setupTempProject(t)
	in := &State{
		LastRunAt: "2026-04-29T14:32:00Z",
		SignalHashes: map[string]string{
			"z": "sha256:zzz",
			"a": "sha256:aaa",
			"m": "sha256:mmm",
		},
		Files: map[string]string{
			"b/file.md": "sha256:bbb",
			"a/file.md": "sha256:aaa",
		},
	}
	if err := Save(dir, in); err != nil {
		t.Fatalf("Save: %v", err)
	}
	first, err := os.ReadFile(FilePath(dir))
	if err != nil {
		t.Fatalf("read1: %v", err)
	}
	if err := Save(dir, in); err != nil {
		t.Fatalf("Save2: %v", err)
	}
	second, err := os.ReadFile(FilePath(dir))
	if err != nil {
		t.Fatalf("read2: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("non-stable output:\n--first--\n%s\n--second--\n%s", first, second)
	}
}

func TestLoad_RejectsNewerSchema(t *testing.T) {
	dir := setupTempProject(t)
	bogus := map[string]any{"version": CurrentVersion + 1}
	data, _ := json.Marshal(bogus)
	if err := os.WriteFile(FilePath(dir), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _, err := Load(dir)
	if err == nil {
		t.Fatalf("expected error on newer schema")
	}
}

func TestLoad_CoercesMissingVersionToCurrent(t *testing.T) {
	dir := setupTempProject(t)
	if err := os.WriteFile(FilePath(dir), []byte(`{"last_run_at":"2025-01-01T00:00:00Z"}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	s, _, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if s.Version != CurrentVersion {
		t.Errorf("missing version not coerced: %d", s.Version)
	}
}

func TestLoad_CorruptJSONReturnsError(t *testing.T) {
	dir := setupTempProject(t)
	if err := os.WriteFile(FilePath(dir), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, _, err := Load(dir)
	if err == nil {
		t.Fatalf("expected parse error")
	}
}

func TestHashContent_IsStable(t *testing.T) {
	a := HashContent([]byte("hello"))
	b := HashContent([]byte("hello"))
	if a != b {
		t.Errorf("hash unstable: %s vs %s", a, b)
	}
	c := HashContent([]byte("hello!"))
	if a == c {
		t.Errorf("hash collision on different content")
	}
}

func TestIsFresh(t *testing.T) {
	cases := []struct {
		name    string
		state   *State
		current map[string]string
		want    bool
	}{
		{"nil state", nil, map[string]string{"a": "x"}, false},
		{"empty signals", &State{}, map[string]string{"a": "x"}, false},
		{
			"all match",
			&State{SignalHashes: map[string]string{"a": "x", "b": "y"}},
			map[string]string{"a": "x", "b": "y"},
			true,
		},
		{
			"one mismatch",
			&State{SignalHashes: map[string]string{"a": "x", "b": "y"}},
			map[string]string{"a": "x", "b": "DIFFERENT"},
			false,
		},
		{
			"new signal not recorded",
			&State{SignalHashes: map[string]string{"a": "x"}},
			map[string]string{"a": "x", "newone": "y"},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := IsFresh(tc.state, tc.current)
			if got != tc.want {
				t.Errorf("IsFresh = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDriftReasons(t *testing.T) {
	s := &State{SignalHashes: map[string]string{
		"a": "sha256:1",
		"b": "sha256:2",
		"c": "sha256:3",
	}}

	t.Run("nil state -> never-refined", func(t *testing.T) {
		got := DriftReasons(nil, map[string]string{"a": "sha256:1"})
		if !reflect.DeepEqual(got, []string{"never-refined"}) {
			t.Errorf("got %v", got)
		}
	})

	t.Run("matching", func(t *testing.T) {
		got := DriftReasons(s, map[string]string{"a": "sha256:1", "b": "sha256:2", "c": "sha256:3"})
		if len(got) != 0 {
			t.Errorf("expected empty, got %v", got)
		}
	})

	t.Run("one changed + one new", func(t *testing.T) {
		got := DriftReasons(s, map[string]string{
			"a":    "sha256:1",
			"b":    "sha256:CHANGED",
			"c":    "sha256:3",
			"newd": "sha256:4",
		})
		sort.Strings(got)
		want := []string{"b", "newd:new"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v want %v", got, want)
		}
	})
}

func TestNoteNudgeFired_IncrementsAndThrottles(t *testing.T) {
	s := &State{}
	for i := 1; i < NudgeThrottleAt; i++ {
		ns := NoteNudgeFired(s, "sig-A")
		if ns.UnactionedCount != i {
			t.Fatalf("count=%d want %d", ns.UnactionedCount, i)
		}
		if ns.Throttled {
			t.Fatalf("throttled too early at %d", i)
		}
	}
	ns := NoteNudgeFired(s, "sig-A")
	if !ns.Throttled {
		t.Errorf("expected throttle at %d, got %+v", NudgeThrottleAt, ns)
	}
}

func TestNoteNudgeFired_NewSignatureResets(t *testing.T) {
	s := &State{
		Nudges: NudgeState{UnactionedCount: 5, Throttled: true, LastSignature: "old"},
	}
	ns := NoteNudgeFired(s, "new")
	if ns.UnactionedCount != 1 {
		t.Errorf("expected reset to 1, got %d", ns.UnactionedCount)
	}
	if ns.Throttled {
		t.Errorf("expected throttled=false after sig change")
	}
	if ns.LastSignature != "new" {
		t.Errorf("LastSignature=%q", ns.LastSignature)
	}
}

func TestResetNudges_Clears(t *testing.T) {
	s := &State{
		Nudges: NudgeState{UnactionedCount: 9, Throttled: true, LastSignature: "x"},
	}
	ResetNudges(s)
	if s.Nudges != (NudgeState{}) {
		t.Errorf("expected zero, got %+v", s.Nudges)
	}
}
