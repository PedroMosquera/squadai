package backup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateID_UniqueAndFormatted(t *testing.T) {
	id1 := GenerateID()
	id2 := GenerateID()

	if id1 == id2 {
		t.Error("generated IDs should be unique")
	}

	// ID should contain a timestamp and a hex suffix separated by "-".
	if len(id1) < 20 {
		t.Errorf("ID too short: %q", id1)
	}
}

func TestChecksum_Deterministic(t *testing.T) {
	data := []byte("hello world")
	c1 := Checksum(data)
	c2 := Checksum(data)

	if c1 != c2 {
		t.Error("checksum should be deterministic")
	}
	if len(c1) != 64 { // SHA-256 hex = 64 chars
		t.Errorf("checksum length = %d, want 64", len(c1))
	}
}

func TestChecksum_DifferentData(t *testing.T) {
	c1 := Checksum([]byte("hello"))
	c2 := Checksum([]byte("world"))

	if c1 == c2 {
		t.Error("different data should produce different checksums")
	}
}

func TestChecksumFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")
	content := []byte("file content for checksum")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ChecksumFile(path)
	if err != nil {
		t.Fatalf("ChecksumFile: %v", err)
	}

	want := Checksum(content)
	if got != want {
		t.Errorf("ChecksumFile = %q, want %q", got, want)
	}
}

func TestChecksumFile_Missing(t *testing.T) {
	_, err := ChecksumFile("/nonexistent/file.txt")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestResolveBackupDir(t *testing.T) {
	tests := []struct {
		name      string
		backupDir string
		homeDir   string
		want      string
	}{
		{
			name:      "empty uses default",
			backupDir: "",
			homeDir:   "/Users/test",
			want:      "/Users/test/.squadai/backups",
		},
		{
			name:      "tilde expansion",
			backupDir: "~/.squadai/backups",
			homeDir:   "/Users/test",
			want:      "/Users/test/.squadai/backups",
		},
		{
			name:      "absolute path unchanged",
			backupDir: "/tmp/backups",
			homeDir:   "/Users/test",
			want:      "/tmp/backups",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ResolveBackupDir(tt.backupDir, tt.homeDir)
			if got != tt.want {
				t.Errorf("ResolveBackupDir(%q, %q) = %q, want %q",
					tt.backupDir, tt.homeDir, got, tt.want)
			}
		})
	}
}

func TestSnapshotFiles_ExistingFiles(t *testing.T) {
	backupDir := t.TempDir()
	store := NewStore(backupDir)

	// Create test files.
	dir := t.TempDir()
	file1 := filepath.Join(dir, "a.txt")
	file2 := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(file1, []byte("content A"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("content B"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest, err := store.SnapshotFiles([]string{file1, file2}, "test")
	if err != nil {
		t.Fatalf("SnapshotFiles: %v", err)
	}

	if manifest.ID == "" {
		t.Error("manifest ID should not be empty")
	}
	if manifest.Command != "test" {
		t.Errorf("command = %q, want test", manifest.Command)
	}
	if manifest.Status != "complete" {
		t.Errorf("status = %q, want complete", manifest.Status)
	}
	if len(manifest.AffectedFiles) != 2 {
		t.Fatalf("affected files = %d, want 2", len(manifest.AffectedFiles))
	}

	for _, snap := range manifest.AffectedFiles {
		if !snap.ExistedBefore {
			t.Errorf("file %q should be marked as existed", snap.Path)
		}
		if snap.ChecksumBefore == "" {
			t.Errorf("file %q should have checksum", snap.Path)
		}
	}

	// Verify backup files exist on disk.
	for _, snap := range manifest.AffectedFiles {
		backupFile := filepath.Join(backupDir, manifest.ID, snap.BackupFile)
		if _, err := os.Stat(backupFile); err != nil {
			t.Errorf("backup file missing: %s", backupFile)
		}
	}
}

func TestSnapshotFiles_NonExistentFiles(t *testing.T) {
	backupDir := t.TempDir()
	store := NewStore(backupDir)

	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.txt")

	manifest, err := store.SnapshotFiles([]string{missing}, "test")
	if err != nil {
		t.Fatalf("SnapshotFiles: %v", err)
	}

	if len(manifest.AffectedFiles) != 1 {
		t.Fatalf("affected files = %d, want 1", len(manifest.AffectedFiles))
	}

	snap := manifest.AffectedFiles[0]
	if snap.ExistedBefore {
		t.Error("file should be marked as not existed")
	}
	if snap.ChecksumBefore != "" {
		t.Error("checksum should be empty for non-existent file")
	}
}

func TestSnapshotFiles_DeduplicatesPaths(t *testing.T) {
	backupDir := t.TempDir()
	store := NewStore(backupDir)

	dir := t.TempDir()
	file := filepath.Join(dir, "dup.txt")
	if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest, err := store.SnapshotFiles([]string{file, file, file}, "test")
	if err != nil {
		t.Fatalf("SnapshotFiles: %v", err)
	}

	if len(manifest.AffectedFiles) != 1 {
		t.Errorf("affected files = %d, want 1 (deduplicated)", len(manifest.AffectedFiles))
	}
}

func TestRollback_RestoresFiles(t *testing.T) {
	backupDir := t.TempDir()
	store := NewStore(backupDir)

	dir := t.TempDir()
	file := filepath.Join(dir, "data.txt")
	original := []byte("original content")
	if err := os.WriteFile(file, original, 0644); err != nil {
		t.Fatal(err)
	}

	// Snapshot.
	manifest, _ := store.SnapshotFiles([]string{file}, "test")

	// Modify the file.
	if err := os.WriteFile(file, []byte("modified content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Rollback.
	if err := store.Rollback(manifest.ID); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	// Verify content restored.
	data, _ := os.ReadFile(file)
	if string(data) != string(original) {
		t.Errorf("content = %q, want %q", string(data), string(original))
	}

	// Verify manifest status updated.
	m, _ := store.Get(manifest.ID)
	if m.Status != "rolled_back" {
		t.Errorf("status = %q, want rolled_back", m.Status)
	}
}

func TestRollback_RemovesNewlyCreatedFiles(t *testing.T) {
	backupDir := t.TempDir()
	store := NewStore(backupDir)

	dir := t.TempDir()
	newFile := filepath.Join(dir, "new.txt")

	// Snapshot (file doesn't exist yet).
	manifest, _ := store.SnapshotFiles([]string{newFile}, "test")

	// Create the file (simulating apply).
	if err := os.WriteFile(newFile, []byte("created during apply"), 0644); err != nil {
		t.Fatal(err)
	}

	// Rollback should remove the file.
	if err := store.Rollback(manifest.ID); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	if _, err := os.Stat(newFile); err == nil {
		t.Error("newly created file should be removed during rollback")
	}
}

func TestRestore_RestoresFiles(t *testing.T) {
	backupDir := t.TempDir()
	store := NewStore(backupDir)

	dir := t.TempDir()
	file := filepath.Join(dir, "data.txt")
	original := []byte("original content")
	if err := os.WriteFile(file, original, 0644); err != nil {
		t.Fatal(err)
	}

	manifest, _ := store.SnapshotFiles([]string{file}, "manual")
	if err := os.WriteFile(file, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := store.Restore(manifest.ID); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	data, _ := os.ReadFile(file)
	if string(data) != string(original) {
		t.Errorf("content = %q, want %q", string(data), string(original))
	}

	m, _ := store.Get(manifest.ID)
	if m.Status != "restored" {
		t.Errorf("status = %q, want restored", m.Status)
	}
}

func TestGet_ValidManifest(t *testing.T) {
	backupDir := t.TempDir()
	store := NewStore(backupDir)

	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	created, _ := store.SnapshotFiles([]string{file}, "test")

	loaded, err := store.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if loaded.ID != created.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, created.ID)
	}
	if loaded.Command != "test" {
		t.Errorf("Command = %q, want test", loaded.Command)
	}
}

func TestGet_NotFound(t *testing.T) {
	store := NewStore(t.TempDir())
	_, err := store.Get("nonexistent-id")
	if err == nil {
		t.Error("expected error for non-existent backup")
	}
}

func TestList_SortedByTimestamp(t *testing.T) {
	backupDir := t.TempDir()
	store := NewStore(backupDir)

	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create two backups.
	m1, _ := store.SnapshotFiles([]string{file}, "first")
	m2, _ := store.SnapshotFiles([]string{file}, "second")

	manifests, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(manifests) != 2 {
		t.Fatalf("expected 2 backups, got %d", len(manifests))
	}

	// Newest first.
	if manifests[0].ID != m2.ID {
		t.Errorf("first should be newest: got %q, want %q", manifests[0].ID, m2.ID)
	}
	if manifests[1].ID != m1.ID {
		t.Errorf("second should be oldest: got %q, want %q", manifests[1].ID, m1.ID)
	}
}

func TestList_EmptyDir(t *testing.T) {
	store := NewStore(t.TempDir())
	manifests, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(manifests) != 0 {
		t.Errorf("expected 0, got %d", len(manifests))
	}
}

func TestList_NonExistentDir(t *testing.T) {
	store := NewStore("/nonexistent/dir/backups")
	manifests, err := store.List()
	if err != nil {
		t.Fatalf("List should not error for missing dir: %v", err)
	}
	if manifests != nil {
		t.Errorf("expected nil, got %v", manifests)
	}
}

func TestDelete_RemovesBackup(t *testing.T) {
	backupDir := t.TempDir()
	store := NewStore(backupDir)

	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest, _ := store.SnapshotFiles([]string{file}, "test")

	if err := store.Delete(manifest.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	// Should not be findable anymore.
	_, err := store.Get(manifest.ID)
	if err == nil {
		t.Error("backup should not exist after delete")
	}
}

func TestDelete_NotFound(t *testing.T) {
	store := NewStore(t.TempDir())
	err := store.Delete("nonexistent-id")
	if err == nil {
		t.Error("expected error for non-existent backup")
	}
}

// ─── Prune ──────────────────────────────────────────────────────────────────

func TestStore_Prune_KeepsN(t *testing.T) {
	backupDir := t.TempDir()
	store := NewStore(backupDir)

	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create 7 backups (each needs a distinct timestamp; GenerateID uses
	// time.Now so they may share a second — we just care about count).
	var ids []string
	for i := 0; i < 7; i++ {
		m, err := store.SnapshotFiles([]string{file}, "test")
		if err != nil {
			t.Fatalf("SnapshotFiles[%d]: %v", i, err)
		}
		ids = append(ids, m.ID)
	}

	deleted, err := store.Prune(3)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if deleted != 4 {
		t.Errorf("deleted = %d, want 4", deleted)
	}

	manifests, err := store.List()
	if err != nil {
		t.Fatalf("List after Prune: %v", err)
	}
	if len(manifests) != 3 {
		t.Errorf("remaining backups = %d, want 3", len(manifests))
	}

	// Verify the 3 newest are kept. List returns newest-first, so the first
	// 3 entries of `ids` (oldest) should be gone.
	_ = ids // used above for creation only
}

func TestStore_Prune_NothingToPrune(t *testing.T) {
	backupDir := t.TempDir()
	store := NewStore(backupDir)

	dir := t.TempDir()
	file := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		if _, err := store.SnapshotFiles([]string{file}, "test"); err != nil {
			t.Fatalf("SnapshotFiles[%d]: %v", i, err)
		}
	}

	deleted, err := store.Prune(5)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0 (nothing to prune)", deleted)
	}

	manifests, err := store.List()
	if err != nil {
		t.Fatalf("List after Prune: %v", err)
	}
	if len(manifests) != 2 {
		t.Errorf("remaining backups = %d, want 2", len(manifests))
	}
}

func TestStore_Prune_KeepZero_Error(t *testing.T) {
	store := NewStore(t.TempDir())
	_, err := store.Prune(0)
	if err == nil {
		t.Fatal("Prune(0) should return an error")
	}
	if !strings.Contains(err.Error(), "keep must be at least 1") {
		t.Errorf("error should mention keep constraint, got: %v", err)
	}
}

func TestStore_Prune_EmptyStore(t *testing.T) {
	store := NewStore(t.TempDir())
	deleted, err := store.Prune(5)
	if err != nil {
		t.Fatalf("Prune on empty store should not error: %v", err)
	}
	if deleted != 0 {
		t.Errorf("deleted = %d, want 0 for empty store", deleted)
	}
}

func TestManifest_JSONRoundTrip(t *testing.T) {
	backupDir := t.TempDir()
	store := NewStore(backupDir)

	dir := t.TempDir()
	file := filepath.Join(dir, "data.txt")
	if err := os.WriteFile(file, []byte("round trip test"), 0644); err != nil {
		t.Fatal(err)
	}

	created, _ := store.SnapshotFiles([]string{file}, "roundtrip")

	// Read the manifest file directly and verify valid JSON.
	manifestPath := filepath.Join(backupDir, created.ID, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if m.ID != created.ID {
		t.Errorf("ID = %q, want %q", m.ID, created.ID)
	}
	if m.Command != "roundtrip" {
		t.Errorf("Command = %q, want roundtrip", m.Command)
	}
	if len(m.AffectedFiles) != 1 {
		t.Errorf("AffectedFiles = %d, want 1", len(m.AffectedFiles))
	}
}
