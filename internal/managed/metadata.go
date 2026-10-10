package managed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/PedroMosquera/squadai/internal/fileutil"
)

const (
	// sidecarDir is the directory under the project root that holds SquadAI
	// local-only tracking files.
	sidecarDir = ".squadai"

	// sidecarFile is the file name for the centralized managed-keys sidecar.
	sidecarFile = "managed.json"

	// userSidecarFile, under ~/.squadai, records files outside the project
	// (agent config under HOME) keyed by absolute path. They cannot go in the
	// project sidecar: it is committed, and a home path is private and differs
	// on every machine.
	userSidecarFile = "managed-home.json"
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
	// ManagedEntries narrows ownership below a top-level key to named
	// children (MCP server names under "mcpServers"). A key missing here was
	// recorded before per-entry tracking existed, so callers must not read
	// its absence as "owns nothing".
	ManagedEntries map[string][]string `json:"managed_entries,omitempty"`
}

// mu guards concurrent reads and writes to the sidecar file within a single
// process. Cross-process safety is provided by the atomic rename in WriteAtomic.
var mu sync.Mutex

// SidecarPath returns the absolute path to the centralized managed-keys sidecar
// file for the given project root.
func SidecarPath(projectRoot string) string {
	return filepath.Join(projectRoot, sidecarDir, sidecarFile)
}

// ReadManagedKeys reads the managed keys recorded for configFile. Every
// function taking a file path accepts it absolute or relative to projectRoot,
// and uses the project sidecar when the file is inside the project and the
// per-user sidecar otherwise. If the sidecar does not exist, or the config
// file has no entry, an empty slice is returned without error.
func ReadManagedKeys(projectRoot, configFile string) ([]string, error) {
	mu.Lock()
	defer mu.Unlock()

	doc, _, key, err := loadFor(projectRoot, configFile)
	if err != nil {
		return nil, err
	}

	entry, ok := doc.ManagedFiles[key]
	if !ok {
		return nil, nil
	}

	out := make([]string, len(entry.ManagedKeys))
	copy(out, entry.ManagedKeys)
	return out, nil
}

// WriteManagedKeys atomically updates (or creates) the sidecar file so that the
// entry for configFile contains exactly keys (sorted).
func WriteManagedKeys(projectRoot, configFile string, keys []string) error {
	mu.Lock()
	defer mu.Unlock()

	doc, file, configFile, err := loadFor(projectRoot, configFile)
	if err != nil {
		return err
	}

	sorted := make([]string, len(keys))
	copy(sorted, keys)
	sort.Strings(sorted)

	if doc.ManagedFiles == nil {
		doc.ManagedFiles = make(map[string]managedFileEntry)
	}
	entry := doc.ManagedFiles[configFile]
	entry.ManagedKeys = sorted
	for key := range entry.ManagedEntries {
		if !containsString(sorted, key) {
			delete(entry.ManagedEntries, key)
		}
	}
	if len(entry.ManagedEntries) == 0 {
		entry.ManagedEntries = nil
	}
	doc.ManagedFiles[configFile] = entry

	return writeDoc(file, doc)
}

// ReadManagedEntries returns the child names SquadAI owns under the top-level
// key of configFile. tracked is false when the sidecar predates per-entry
// tracking for that key (or has no entry at all); the caller then has to
// decide ownership from the key-level record in ReadManagedKeys.
func ReadManagedEntries(projectRoot, configFile, key string) (names []string, tracked bool, err error) {
	mu.Lock()
	defer mu.Unlock()

	doc, _, configFile, err := loadFor(projectRoot, configFile)
	if err != nil {
		return nil, false, err
	}
	stored, ok := doc.ManagedFiles[configFile].ManagedEntries[key]
	if !ok {
		return nil, false, nil
	}
	out := make([]string, len(stored))
	copy(out, stored)
	return out, true, nil
}

// WriteManagedEntries records exactly names (sorted) as the children SquadAI
// owns under key in configFile, and adds key to the file's managed keys in the
// same atomic write. An empty names list is stored as tracked-and-empty.
func WriteManagedEntries(projectRoot, configFile, key string, names []string) error {
	mu.Lock()
	defer mu.Unlock()

	doc, file, configFile, err := loadFor(projectRoot, configFile)
	if err != nil {
		return err
	}

	sorted := make([]string, len(names))
	copy(sorted, names)
	sort.Strings(sorted)

	if doc.ManagedFiles == nil {
		doc.ManagedFiles = make(map[string]managedFileEntry)
	}
	entry := doc.ManagedFiles[configFile]
	if !containsString(entry.ManagedKeys, key) {
		entry.ManagedKeys = append(entry.ManagedKeys, key)
		sort.Strings(entry.ManagedKeys)
	}
	if entry.ManagedEntries == nil {
		entry.ManagedEntries = make(map[string][]string)
	}
	entry.ManagedEntries[key] = sorted
	doc.ManagedFiles[configFile] = entry

	return writeDoc(file, doc)
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// RemoveManagedFile drops the managed-keys entry for configFile so drift
// checks stop expecting the file. No-op if no entry exists.
func RemoveManagedFile(projectRoot, configFile string) error {
	mu.Lock()
	defer mu.Unlock()

	doc, file, configFile, err := loadFor(projectRoot, configFile)
	if err != nil {
		return err
	}
	if _, ok := doc.ManagedFiles[configFile]; !ok {
		return nil
	}
	delete(doc.ManagedFiles, configFile)
	return writeDoc(file, doc)
}

// locate returns the sidecar file that records path and the key path is
// stored under there.
func locate(projectRoot, path string) (file, key string, err error) {
	if projectRoot == "" {
		return SidecarPath(projectRoot), path, nil
	}
	abs := path
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(projectRoot, abs)
	}
	if rel, relErr := filepath.Rel(projectRoot, abs); relErr == nil && insideProject(rel) {
		return SidecarPath(projectRoot), rel, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", fmt.Errorf("resolve user sidecar for %s: %w", path, err)
	}
	return filepath.Join(home, sidecarDir, userSidecarFile), filepath.Clean(abs), nil
}

func insideProject(rel string) bool {
	return !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// loadFor reads the sidecar that records path; file and key are where a
// write of path's record belongs.
func loadFor(projectRoot, path string) (doc sidecarDoc, file, key string, err error) {
	file, key, err = locate(projectRoot, path)
	if err != nil {
		return sidecarDoc{}, "", "", err
	}
	if file == SidecarPath(projectRoot) {
		doc, err = readSidecar(projectRoot)
	} else {
		doc, err = readDoc(file)
	}
	return doc, file, key, err
}

// readSidecar reads the project sidecar, discarding records of paths outside
// the project. Older versions wrote home files there as "../" paths that only
// resolve on the machine that wrote them. Ignoring them means such a file is
// stripped rather than deleted, and the next write drops them.
func readSidecar(projectRoot string) (sidecarDoc, error) {
	doc, err := readDoc(SidecarPath(projectRoot))
	if err != nil {
		return doc, err
	}
	var created []string
	for _, f := range doc.CreatedFiles {
		if insideProject(f) {
			created = append(created, f)
		}
	}
	doc.CreatedFiles = created
	for f := range doc.ManagedFiles {
		if !insideProject(f) {
			delete(doc.ManagedFiles, f)
		}
	}
	return doc, nil
}

// readDoc returns an empty doc if the file does not exist; returns an error
// for any other I/O or parse failure.
func readDoc(path string) (sidecarDoc, error) {
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
// Idempotent: calling twice does not duplicate the entry.
func TrackCreatedFile(projectRoot, relPath string) error {
	mu.Lock()
	defer mu.Unlock()

	doc, file, relPath, err := loadFor(projectRoot, relPath)
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

	return writeDoc(file, doc)
}

// UntrackCreatedFile removes relPath from the created-files list.
// No-op if not present.
func UntrackCreatedFile(projectRoot, relPath string) error {
	mu.Lock()
	defer mu.Unlock()

	doc, file, relPath, err := loadFor(projectRoot, relPath)
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

	return writeDoc(file, doc)
}

// ListManagedFiles returns the files inside the project that have managed
// entries (keys of managed_files), sorted. Returns an empty slice (not nil) when the sidecar
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

// ListCreatedFiles returns the files inside the project that SquadAI created
// (relative paths), sorted. Files it created under HOME are shared by every
// project and are not listed. Returns an empty slice (not nil) when the
// sidecar does not exist.
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

func writeSidecar(projectRoot string, doc sidecarDoc) error {
	return writeDoc(SidecarPath(projectRoot), doc)
}

func writeDoc(path string, doc sidecarDoc) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sidecar: %w", err)
	}
	data = append(data, '\n')

	if _, err := fileutil.WriteAtomic(path, data, 0644); err != nil {
		return fmt.Errorf("write sidecar: %w", err)
	}

	return nil
}
