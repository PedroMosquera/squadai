package managed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/PedroMosquera/squadai/internal/fileutil"
)

const (
	// sidecarDir is the directory under the project root that holds SquadAI
	// local-only tracking files.
	sidecarDir = ".squadai"

	// sidecarFile is the file name for the centralized managed-keys sidecar.
	sidecarFile = "managed.json"
)

// sidecarDoc is the on-disk structure of the sidecar file.
type sidecarDoc struct {
	Scope        string                      `json:"scope,omitempty"`
	ManagedFiles map[string]managedFileEntry `json:"managed_files"`
	CreatedFiles []string                    `json:"created_files,omitempty"`
}

// managedFileEntry tracks which keys are managed for a single config file.
type managedFileEntry struct {
	ManagedKeys []string `json:"managed_keys"`
}

// mu guards concurrent reads and writes to the sidecar file within a single
// process. Cross-process safety is provided by the atomic rename in WriteAtomic.
var mu sync.Mutex

// SidecarPath returns the absolute path to the centralized managed-keys sidecar
// file for the given project root.
func SidecarPath(projectRoot string) string {
	return filepath.Join(projectRoot, sidecarDir, sidecarFile)
}

// ReadManagedKeys reads the managed keys for the given config file (expressed
// as a relative path from the project root) from the centralized sidecar.
// If the sidecar does not exist, or the config file has no entry, an empty
// slice is returned without error.
func ReadManagedKeys(projectRoot, configFile string) ([]string, error) {
	mu.Lock()
	defer mu.Unlock()

	doc, err := readSidecar(projectRoot)
	if err != nil {
		return nil, err
	}

	entry, ok := doc.ManagedFiles[configFile]
	if !ok {
		return nil, nil
	}

	out := make([]string, len(entry.ManagedKeys))
	copy(out, entry.ManagedKeys)
	return out, nil
}

// WriteManagedKeys atomically updates (or creates) the sidecar file so that the
// entry for configFile contains exactly keys (sorted). configFile must be a
// relative path from the project root.
func WriteManagedKeys(projectRoot, configFile string, keys []string) error {
	mu.Lock()
	defer mu.Unlock()

	doc, err := readSidecar(projectRoot)
	if err != nil {
		return err
	}

	sorted := make([]string, len(keys))
	copy(sorted, keys)
	sort.Strings(sorted)

	if doc.ManagedFiles == nil {
		doc.ManagedFiles = make(map[string]managedFileEntry)
	}
	doc.ManagedFiles[configFile] = managedFileEntry{ManagedKeys: sorted}

	return writeSidecar(projectRoot, doc)
}

// readSidecar reads the sidecar file. Returns an empty doc if the file does not
// exist; returns an error for any other I/O or parse failure.
func readSidecar(projectRoot string) (sidecarDoc, error) {
	path := SidecarPath(projectRoot)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return sidecarDoc{}, nil
		}
		return sidecarDoc{}, fmt.Errorf("read sidecar: %w", err)
	}

	var doc sidecarDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return sidecarDoc{}, fmt.Errorf("parse sidecar: %w", err)
	}

	return doc, nil
}

// TrackCreatedFile records that SquadAI created the file at relPath.
// Idempotent — calling twice does not duplicate the entry.
func TrackCreatedFile(projectRoot, relPath string) error {
	mu.Lock()
	defer mu.Unlock()

	doc, err := readSidecar(projectRoot)
	if err != nil {
		return err
	}

	for _, f := range doc.CreatedFiles {
		if f == relPath {
			return nil // already tracked
		}
	}

	doc.CreatedFiles = append(doc.CreatedFiles, relPath)
	sort.Strings(doc.CreatedFiles)

	return writeSidecar(projectRoot, doc)
}

// UntrackCreatedFile removes relPath from the created-files list.
// No-op if not present.
func UntrackCreatedFile(projectRoot, relPath string) error {
	mu.Lock()
	defer mu.Unlock()

	doc, err := readSidecar(projectRoot)
	if err != nil {
		return err
	}

	filtered := doc.CreatedFiles[:0]
	for _, f := range doc.CreatedFiles {
		if f != relPath {
			filtered = append(filtered, f)
		}
	}
	if len(filtered) == len(doc.CreatedFiles) {
		return nil // not present, no-op
	}
	doc.CreatedFiles = filtered

	return writeSidecar(projectRoot, doc)
}

// ListManagedFiles returns all files that have managed marker blocks (keys of
// managed_files), sorted. Returns an empty slice (not nil) when the sidecar
// does not exist or no entries are recorded.
func ListManagedFiles(projectRoot string) ([]string, error) {
	mu.Lock()
	defer mu.Unlock()

	doc, err := readSidecar(projectRoot)
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(doc.ManagedFiles))
	for f := range doc.ManagedFiles {
		out = append(out, f)
	}
	sort.Strings(out)
	return out, nil
}

// ListCreatedFiles returns all files SquadAI created (relative paths), sorted.
// Returns an empty slice (not nil) when the sidecar does not exist.
func ListCreatedFiles(projectRoot string) ([]string, error) {
	mu.Lock()
	defer mu.Unlock()

	doc, err := readSidecar(projectRoot)
	if err != nil {
		return nil, err
	}

	out := make([]string, len(doc.CreatedFiles))
	copy(out, doc.CreatedFiles)
	return out, nil
}

// SetScope records the scope ("global" or "repo") in the sidecar.
func SetScope(projectRoot, scope string) error {
	mu.Lock()
	defer mu.Unlock()

	doc, err := readSidecar(projectRoot)
	if err != nil {
		return err
	}

	doc.Scope = scope
	return writeSidecar(projectRoot, doc)
}

// GetScope returns the scope from the sidecar, defaulting to "repo".
func GetScope(projectRoot string) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	doc, err := readSidecar(projectRoot)
	if err != nil {
		return "", err
	}

	if doc.Scope == "" {
		return "repo", nil
	}
	return doc.Scope, nil
}

// DeleteSidecar removes the .squadai/managed.json file and the .squadai/ dir
// if it becomes empty afterwards.
func DeleteSidecar(projectRoot string) error {
	mu.Lock()
	defer mu.Unlock()

	path := SidecarPath(projectRoot)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete sidecar: %w", err)
	}

	dir := filepath.Join(projectRoot, sidecarDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read sidecar dir: %w", err)
	}
	if len(entries) == 0 {
		if err := os.Remove(dir); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove sidecar dir: %w", err)
		}
	}
	return nil
}

// writeSidecar marshals doc and writes it atomically to the sidecar path.
func writeSidecar(projectRoot string, doc sidecarDoc) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sidecar: %w", err)
	}
	data = append(data, '\n')

	path := SidecarPath(projectRoot)
	if _, err := fileutil.WriteAtomic(path, data, 0644); err != nil {
		return fmt.Errorf("write sidecar: %w", err)
	}

	return nil
}
