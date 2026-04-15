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
	ManagedFiles map[string]managedFileEntry `json:"managed_files"`
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
