package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PedroMosquera/squadai/internal/fileutil"
)

// GCResult reports what was archived.
type GCResult struct {
	Archived  []string `json:"archived"`
	Remaining int      `json:"remaining"`
	DryRun    bool     `json:"dry_run"`
}

// GC archives unreferenced notes older than the given duration to
// docs/memory/.archive/ and prunes them from the live index.
// Notes referenced in decisions/ ADR files are exempt.
// If dryRun is true, reports what would be archived without moving anything.
func GC(projectDir string, olderThan time.Duration, dryRun bool) (*GCResult, error) {
	memoryDir := filepath.Join(projectDir, "docs", "memory")
	result := &GCResult{DryRun: dryRun}

	// Pre-read decisions/ contents so each stale note can be checked for
	// references without re-reading on every visit.
	decisionTexts := decisionsContents(memoryDir)
	cutoff := time.Now().Add(-olderThan)

	var toArchive []string // relative to projectDir

	walkErr := filepath.Walk(memoryDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable
		}
		if info.IsDir() {
			name := info.Name()
			if name == "_inbox" || name == ".archive" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		if info.Name() == "README.md" {
			return nil
		}
		// Stale = mod time older than the cutoff.
		if !info.ModTime().Before(cutoff) {
			return nil
		}
		// Exempt notes referenced by an ADR in decisions/.
		if isReferenced(info.Name(), decisionTexts) {
			return nil
		}

		rel, relErr := filepath.Rel(projectDir, path)
		if relErr != nil {
			rel = path
		}
		toArchive = append(toArchive, rel)
		return nil
	})
	if walkErr != nil && !os.IsNotExist(walkErr) {
		return nil, walkErr
	}

	result.Archived = toArchive

	if dryRun {
		result.Remaining = countLiveNotes(memoryDir) - len(toArchive)
		return result, nil
	}

	// Move each stale note into docs/memory/.archive/ preserving subdirs.
	archiveDir := filepath.Join(memoryDir, ".archive")
	for _, rel := range toArchive {
		src := filepath.Join(projectDir, rel)
		subRel, subErr := filepath.Rel(memoryDir, src)
		if subErr != nil {
			return nil, subErr
		}
		dest := filepath.Join(archiveDir, subRel)
		if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
			return nil, fmt.Errorf("create archive dir: %w", err)
		}
		if err := os.Rename(src, dest); err != nil {
			return nil, fmt.Errorf("move file to archive: %w", err)
		}
	}

	// Rebuild the live index, then strip any archived entries it may have
	// picked up (Reindex does not skip .archive/).
	if _, err := Reindex(projectDir); err != nil {
		return nil, fmt.Errorf("reindex after gc: %w", err)
	}
	if err := pruneArchivedFromIndex(projectDir); err != nil {
		return nil, fmt.Errorf("prune archived from index: %w", err)
	}

	result.Remaining = countLiveNotes(memoryDir)
	return result, nil
}

// decisionsContents returns the full text of every file in
// docs/memory/decisions/. Used for reference-exemption checks.
func decisionsContents(memoryDir string) []string {
	var texts []string
	decisionsDir := filepath.Join(memoryDir, "decisions")
	entries, err := os.ReadDir(decisionsDir)
	if err != nil {
		return texts
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, readErr := os.ReadFile(filepath.Join(decisionsDir, e.Name()))
		if readErr != nil {
			continue
		}
		texts = append(texts, string(data))
	}
	return texts
}

// isReferenced reports whether filename appears as a substring in any of the
// provided decision texts.
func isReferenced(filename string, decisionTexts []string) bool {
	for _, t := range decisionTexts {
		if strings.Contains(t, filename) {
			return true
		}
	}
	return false
}

// countLiveNotes counts .md notes under docs/memory/, excluding _inbox/,
// .archive/, and README.md.
func countLiveNotes(memoryDir string) int {
	count := 0
	_ = filepath.Walk(memoryDir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == "_inbox" || name == ".archive" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".md") && info.Name() != "README.md" {
			count++
		}
		return nil
	})
	return count
}

// pruneArchivedFromIndex rewrites the live index, dropping any entry whose
// path lives under docs/memory/.archive/.
func pruneArchivedFromIndex(projectDir string) error {
	idx, err := LoadIndex(projectDir)
	if err != nil {
		return err
	}
	archiveSeg := "/.archive/"
	var kept []IndexEntry
	for _, e := range idx.Entries {
		if strings.Contains(filepath.ToSlash(e.Path), archiveSeg) {
			continue
		}
		kept = append(kept, e)
	}
	idx.Entries = kept

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	if _, err := fileutil.WriteAtomic(filepath.Join(projectDir, indexPath), data, 0644); err != nil {
		return err
	}
	return nil
}
