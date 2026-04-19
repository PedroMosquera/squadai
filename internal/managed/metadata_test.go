package managed

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// ─── SidecarPath ─────────────────────────────────────────────────────────────

func TestSidecarPath_ReturnsExpectedLocation(t *testing.T) {
	got := SidecarPath("/project/root")
	want := "/project/root/.squadai/managed.json"
	if got != want {
		t.Errorf("SidecarPath = %q, want %q", got, want)
	}
}

// ─── ReadManagedKeys ──────────────────────────────────────────────────────────

func TestReadManagedKeys_MissingSidecar_ReturnsEmpty(t *testing.T) {
	root := t.TempDir()

	keys, err := ReadManagedKeys(root, "opencode.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected empty keys, got %v", keys)
	}
}

func TestReadManagedKeys_EntryNotFound_ReturnsEmpty(t *testing.T) {
	root := t.TempDir()

	// Write sidecar with a different config file.
	if err := WriteManagedKeys(root, "other.json", []string{"key1"}); err != nil {
		t.Fatal(err)
	}

	keys, err := ReadManagedKeys(root, "opencode.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 0 {
		t.Errorf("expected empty keys for absent entry, got %v", keys)
	}
}

func TestReadManagedKeys_ReturnsStoredKeys(t *testing.T) {
	root := t.TempDir()

	if err := WriteManagedKeys(root, "opencode.json", []string{"model", "mcp"}); err != nil {
		t.Fatal(err)
	}

	keys, err := ReadManagedKeys(root, "opencode.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d: %v", len(keys), keys)
	}
	// Keys are returned sorted.
	if keys[0] != "mcp" || keys[1] != "model" {
		t.Errorf("expected [mcp model], got %v", keys)
	}
}

// ─── WriteManagedKeys ─────────────────────────────────────────────────────────

func TestWriteManagedKeys_CreatesSidecarDir(t *testing.T) {
	root := t.TempDir()

	if err := WriteManagedKeys(root, "opencode.json", []string{"model"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := SidecarPath(root)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("sidecar file not created: %v", err)
	}
}

func TestWriteManagedKeys_SortsKeys(t *testing.T) {
	root := t.TempDir()

	// Write unsorted.
	if err := WriteManagedKeys(root, "opencode.json", []string{"zzz", "aaa", "mmm"}); err != nil {
		t.Fatal(err)
	}

	keys, err := ReadManagedKeys(root, "opencode.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
	if keys[0] != "aaa" || keys[1] != "mmm" || keys[2] != "zzz" {
		t.Errorf("keys not sorted: %v", keys)
	}
}

func TestWriteManagedKeys_PreservesOtherEntries(t *testing.T) {
	root := t.TempDir()

	// Write first config file entry.
	if err := WriteManagedKeys(root, "opencode.json", []string{"model"}); err != nil {
		t.Fatal(err)
	}
	// Write second config file entry.
	if err := WriteManagedKeys(root, ".cursor/mcp.json", []string{"mcpServers"}); err != nil {
		t.Fatal(err)
	}

	// First entry should still be there.
	keys1, err := ReadManagedKeys(root, "opencode.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys1) != 1 || keys1[0] != "model" {
		t.Errorf("first entry corrupted: %v", keys1)
	}

	// Second entry should also be there.
	keys2, err := ReadManagedKeys(root, ".cursor/mcp.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys2) != 1 || keys2[0] != "mcpServers" {
		t.Errorf("second entry wrong: %v", keys2)
	}
}

func TestWriteManagedKeys_OverwritesExistingEntry(t *testing.T) {
	root := t.TempDir()

	if err := WriteManagedKeys(root, "opencode.json", []string{"model"}); err != nil {
		t.Fatal(err)
	}
	// Overwrite with a different set.
	if err := WriteManagedKeys(root, "opencode.json", []string{"model", "permission"}); err != nil {
		t.Fatal(err)
	}

	keys, err := ReadManagedKeys(root, "opencode.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys after overwrite, got %d", len(keys))
	}
}

func TestWriteManagedKeys_EmptyKeys(t *testing.T) {
	root := t.TempDir()

	if err := WriteManagedKeys(root, "opencode.json", []string{}); err != nil {
		t.Fatalf("unexpected error with empty keys: %v", err)
	}

	keys, err := ReadManagedKeys(root, "opencode.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Errorf("expected empty keys, got %v", keys)
	}
}

// ─── Concurrent safety ────────────────────────────────────────────────────────

func TestWriteManagedKeys_ConcurrentWrites_NoDataLoss(t *testing.T) {
	root := t.TempDir()

	const workers = 10
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			configFile := filepath.Join("config", "file.json")
			// Each worker writes the same config file with a single key.
			// After all complete, the entry should exist (last writer wins for keys,
			// but the file should not be corrupt).
			if err := WriteManagedKeys(root, configFile, []string{"key"}); err != nil {
				t.Errorf("worker %d: unexpected error: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()

	keys, err := ReadManagedKeys(root, filepath.Join("config", "file.json"))
	if err != nil {
		t.Fatalf("unexpected error after concurrent writes: %v", err)
	}
	if len(keys) != 1 || keys[0] != "key" {
		t.Errorf("unexpected keys after concurrent writes: %v", keys)
	}
}

// ─── Idempotency ─────────────────────────────────────────────────────────────

func TestWriteManagedKeys_Idempotent(t *testing.T) {
	root := t.TempDir()

	for i := 0; i < 3; i++ {
		if err := WriteManagedKeys(root, "opencode.json", []string{"mcp", "model"}); err != nil {
			t.Fatalf("write %d failed: %v", i, err)
		}
	}

	keys, err := ReadManagedKeys(root, "opencode.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Errorf("expected 2 keys after idempotent writes, got %d: %v", len(keys), keys)
	}
}

// ─── TrackCreatedFile ─────────────────────────────────────────────────────────

func TestTrackCreatedFile_Basic(t *testing.T) {
	root := t.TempDir()

	if err := TrackCreatedFile(root, "AGENTS.md"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files, err := ListCreatedFiles(root)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 1 || files[0] != "AGENTS.md" {
		t.Errorf("got %v, want [AGENTS.md]", files)
	}
}

func TestTrackCreatedFile_Idempotent(t *testing.T) {
	root := t.TempDir()

	for i := 0; i < 3; i++ {
		if err := TrackCreatedFile(root, "AGENTS.md"); err != nil {
			t.Fatalf("iteration %d: %v", i, err)
		}
	}

	files, err := ListCreatedFiles(root)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("expected 1 file after idempotent track, got %d: %v", len(files), files)
	}
}

func TestTrackCreatedFile_Multiple(t *testing.T) {
	root := t.TempDir()

	inputs := []string{"CLAUDE.md", "AGENTS.md", ".opencode/agents/reviewer.md"}
	for _, f := range inputs {
		if err := TrackCreatedFile(root, f); err != nil {
			t.Fatalf("track %q: %v", f, err)
		}
	}

	files, err := ListCreatedFiles(root)
	if err != nil {
		t.Fatalf("list: %v", err)
	}

	want := []string{".opencode/agents/reviewer.md", "AGENTS.md", "CLAUDE.md"}
	if len(files) != len(want) {
		t.Fatalf("expected %d files, got %d: %v", len(want), len(files), files)
	}
	for i, w := range want {
		if files[i] != w {
			t.Errorf("files[%d] = %q, want %q", i, files[i], w)
		}
	}
}

// ─── UntrackCreatedFile ───────────────────────────────────────────────────────

func TestUntrackCreatedFile_Exists(t *testing.T) {
	root := t.TempDir()

	if err := TrackCreatedFile(root, "AGENTS.md"); err != nil {
		t.Fatal(err)
	}
	if err := TrackCreatedFile(root, "CLAUDE.md"); err != nil {
		t.Fatal(err)
	}

	if err := UntrackCreatedFile(root, "AGENTS.md"); err != nil {
		t.Fatalf("untrack: %v", err)
	}

	files, err := ListCreatedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "CLAUDE.md" {
		t.Errorf("got %v, want [CLAUDE.md]", files)
	}
}

func TestUntrackCreatedFile_NotPresent(t *testing.T) {
	root := t.TempDir()

	// Calling on missing entry should be a no-op with no error.
	if err := UntrackCreatedFile(root, "nonexistent.md"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	files, err := ListCreatedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty, got %v", files)
	}
}

// ─── ListCreatedFiles ─────────────────────────────────────────────────────────

func TestListCreatedFiles_Empty(t *testing.T) {
	root := t.TempDir()

	files, err := ListCreatedFiles(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty list, got %v", files)
	}
}

// ─── SetScope / GetScope ──────────────────────────────────────────────────────

func TestSetScope_And_GetScope(t *testing.T) {
	root := t.TempDir()

	if err := SetScope(root, "global"); err != nil {
		t.Fatalf("set scope: %v", err)
	}

	got, err := GetScope(root)
	if err != nil {
		t.Fatalf("get scope: %v", err)
	}
	if got != "global" {
		t.Errorf("scope = %q, want global", got)
	}
}

func TestGetScope_Default(t *testing.T) {
	root := t.TempDir()

	got, err := GetScope(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "repo" {
		t.Errorf("default scope = %q, want repo", got)
	}
}

// ─── DeleteSidecar ────────────────────────────────────────────────────────────

func TestDeleteSidecar(t *testing.T) {
	root := t.TempDir()

	if err := TrackCreatedFile(root, "AGENTS.md"); err != nil {
		t.Fatal(err)
	}

	if err := DeleteSidecar(root); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Sidecar file should be gone.
	if _, err := os.Stat(SidecarPath(root)); !os.IsNotExist(err) {
		t.Error("expected sidecar to be removed")
	}

	// Directory should also be gone (was empty after file removal).
	dir := filepath.Join(root, ".squadai")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("expected empty .squadai dir to be removed")
	}
}

func TestDeleteSidecar_NonEmpty(t *testing.T) {
	root := t.TempDir()

	if err := TrackCreatedFile(root, "AGENTS.md"); err != nil {
		t.Fatal(err)
	}

	// Add another file inside .squadai so the dir is non-empty after deletion.
	dir := filepath.Join(root, ".squadai")
	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(other, []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := DeleteSidecar(root); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Sidecar file should be gone.
	if _, err := os.Stat(SidecarPath(root)); !os.IsNotExist(err) {
		t.Error("expected sidecar file to be removed")
	}

	// Directory should remain (other.txt still there).
	if _, err := os.Stat(dir); err != nil {
		t.Errorf("expected .squadai dir to remain, got: %v", err)
	}
}

// ─── Coexistence ─────────────────────────────────────────────────────────────

func TestCreatedFiles_WithManagedKeys(t *testing.T) {
	root := t.TempDir()

	// Write managed keys for one file.
	if err := WriteManagedKeys(root, "opencode.json", []string{"mcp", "model"}); err != nil {
		t.Fatal(err)
	}

	// Track a created file.
	if err := TrackCreatedFile(root, "AGENTS.md"); err != nil {
		t.Fatal(err)
	}

	// Both should coexist without interference.
	keys, err := ReadManagedKeys(root, "opencode.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Errorf("managed keys lost: got %v", keys)
	}

	files, err := ListCreatedFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "AGENTS.md" {
		t.Errorf("created files wrong: got %v", files)
	}
}

// ─── Concurrent safety ────────────────────────────────────────────────────────

func TestConcurrent_TrackCreatedFile(t *testing.T) {
	root := t.TempDir()

	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_ = TrackCreatedFile(root, "AGENTS.md")
		}()
	}
	wg.Wait()

	files, err := ListCreatedFiles(root)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(files) != 1 || files[0] != "AGENTS.md" {
		t.Errorf("expected exactly [AGENTS.md], got %v", files)
	}
}

// ─── ListManagedFiles ─────────────────────────────────────────────────────────

func TestListManagedFiles_MissingSidecar_ReturnsEmpty(t *testing.T) {
	root := t.TempDir()
	files, err := ListManagedFiles(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("expected empty, got %v", files)
	}
}

func TestListManagedFiles_ReturnsSortedPaths(t *testing.T) {
	root := t.TempDir()

	if err := WriteManagedKeys(root, "z-file.md", []string{"k1"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteManagedKeys(root, "a-file.md", []string{"k2"}); err != nil {
		t.Fatal(err)
	}

	files, err := ListManagedFiles(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %v", files)
	}
	if files[0] != "a-file.md" || files[1] != "z-file.md" {
		t.Errorf("expected sorted [a-file.md z-file.md], got %v", files)
	}
}
