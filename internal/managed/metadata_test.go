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
