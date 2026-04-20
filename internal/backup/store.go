package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/PedroMosquera/squadai/internal/fileutil"
)

// Store manages backup manifests and file snapshots on disk.
//
// Storage layout:
//
//	<baseDir>/<id>/manifest.json
//	<baseDir>/<id>/files/0
//	<baseDir>/<id>/files/1
//	...
type Store struct {
	baseDir string
}

// NewStore creates a Store rooted at the given directory.
func NewStore(baseDir string) *Store {
	return &Store{baseDir: baseDir}
}

// SnapshotFiles creates a backup by copying the given file paths.
// Files that don't exist are recorded as non-existent (for rollback
// to know they should be removed). Duplicate paths are deduplicated.
func (s *Store) SnapshotFiles(paths []string, command string) (*Manifest, error) {
	// Deduplicate paths while preserving order.
	seen := make(map[string]bool)
	unique := make([]string, 0, len(paths))
	for _, p := range paths {
		if !seen[p] {
			seen[p] = true
			unique = append(unique, p)
		}
	}

	id, err := GenerateID()
	if err != nil {
		return nil, fmt.Errorf("generate backup id: %w", err)
	}
	backupDir := filepath.Join(s.baseDir, id)
	filesDir := filepath.Join(backupDir, "files")

	if err := os.MkdirAll(filesDir, 0755); err != nil {
		return nil, fmt.Errorf("create backup dir: %w", err)
	}

	manifest := &Manifest{
		ID:            id,
		Timestamp:     time.Now().UTC(),
		Command:       command,
		AffectedFiles: make([]FileSnapshot, 0, len(unique)),
		Status:        "complete",
	}

	for i, path := range unique {
		snap := FileSnapshot{
			Path:       path,
			BackupFile: fmt.Sprintf("files/%d", i),
		}

		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				snap.ExistedBefore = false
				manifest.AffectedFiles = append(manifest.AffectedFiles, snap)
				continue
			}
			// Clean up on failure.
			os.RemoveAll(backupDir)
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		snap.ExistedBefore = true
		snap.ChecksumBefore = Checksum(data)

		dest := filepath.Join(backupDir, snap.BackupFile)
		if _, err := fileutil.WriteAtomic(dest, data, 0644); err != nil {
			os.RemoveAll(backupDir)
			return nil, fmt.Errorf("backup %s: %w", path, err)
		}

		manifest.AffectedFiles = append(manifest.AffectedFiles, snap)
	}

	if err := s.writeManifest(id, manifest); err != nil {
		os.RemoveAll(backupDir)
		return nil, err
	}

	return manifest, nil
}

// Rollback restores all files from a backup and marks the manifest as "rolled_back".
func (s *Store) Rollback(id string) error {
	manifest, err := s.restoreFiles(id)
	if err != nil {
		return err
	}
	manifest.Status = "rolled_back"
	return s.writeManifest(id, manifest)
}

// Restore restores all files from a backup and marks the manifest as "restored".
func (s *Store) Restore(id string) error {
	manifest, err := s.restoreFiles(id)
	if err != nil {
		return err
	}
	manifest.Status = "restored"
	return s.writeManifest(id, manifest)
}

// Get loads a manifest by ID.
func (s *Store) Get(id string) (*Manifest, error) {
	path := filepath.Join(s.baseDir, id, "manifest.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("backup %q not found", id)
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

// List returns all manifests sorted by timestamp (newest first).
func (s *Store) List() ([]Manifest, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read backup dir: %w", err)
	}

	var manifests []Manifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		m, err := s.Get(entry.Name())
		if err != nil {
			continue // skip corrupt entries
		}
		manifests = append(manifests, *m)
	}

	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].Timestamp.After(manifests[j].Timestamp)
	})

	return manifests, nil
}

// Prune removes all but the most recent `keep` backups.
// Returns the number of deleted backups.
// Returns an error if keep < 1.
func (s *Store) Prune(keep int) (int, error) {
	if keep < 1 {
		return 0, fmt.Errorf("keep must be at least 1, got %d", keep)
	}
	manifests, err := s.List()
	if err != nil {
		return 0, err
	}
	if len(manifests) <= keep {
		return 0, nil
	}
	deleted := 0
	for _, m := range manifests[keep:] {
		if err := s.Delete(m.ID); err != nil {
			return deleted, fmt.Errorf("failed to delete backup %s: %w", m.ID, err)
		}
		deleted++
	}
	return deleted, nil
}

// Delete removes a backup and all its files.
func (s *Store) Delete(id string) error {
	dir := filepath.Join(s.baseDir, id)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("backup %q not found", id)
	}
	return os.RemoveAll(dir)
}

// restoreFiles is the shared implementation for Rollback and Restore.
// It restores all files from the backup to their original locations.
func (s *Store) restoreFiles(id string) (*Manifest, error) {
	manifest, err := s.Get(id)
	if err != nil {
		return nil, err
	}

	backupDir := filepath.Join(s.baseDir, id)

	for _, snap := range manifest.AffectedFiles {
		if !snap.ExistedBefore {
			// File was created during the operation — remove it.
			os.Remove(snap.Path)
			continue
		}

		src := filepath.Join(backupDir, snap.BackupFile)
		data, err := os.ReadFile(src)
		if err != nil {
			return nil, fmt.Errorf("read backup for %s: %w", snap.Path, err)
		}

		if err := os.MkdirAll(filepath.Dir(snap.Path), 0755); err != nil {
			return nil, fmt.Errorf("create dir for %s: %w", snap.Path, err)
		}

		if _, err := fileutil.WriteAtomic(snap.Path, data, 0644); err != nil {
			return nil, fmt.Errorf("restore %s: %w", snap.Path, err)
		}
	}

	return manifest, nil
}

// writeManifest persists a manifest to disk.
func (s *Store) writeManifest(id string, m *Manifest) error {
	path := filepath.Join(s.baseDir, id, "manifest.json")
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}
	if _, err := fileutil.WriteAtomic(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}
