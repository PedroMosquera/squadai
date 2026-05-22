package memory

import (
	"os"
	"path/filepath"
	"strings"
)

// MemoryStatus holds counts for the memory status command.
type MemoryStatus struct {
	InboxCount   int `json:"inbox_count"`
	TotalCount   int `json:"total_count"`
	IndexedCount int `json:"indexed_count"`
}

// Status counts inbox notes, total memory notes, and indexed entries.
func Status(projectDir string) (MemoryStatus, error) {
	var s MemoryStatus

	memoryDir := filepath.Join(projectDir, "docs", "memory")

	// Count inbox notes.
	inboxDir := filepath.Join(memoryDir, "_inbox")
	if entries, err := os.ReadDir(inboxDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				s.InboxCount++
			}
		}
	}

	// Count total notes (all .md files in memory dir, excluding inbox).
	err := filepath.Walk(memoryDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			if info.Name() == "_inbox" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(info.Name(), ".md") {
			s.TotalCount++
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return s, err
	}

	// Count indexed entries.
	idx, err := LoadIndex(projectDir)
	if err != nil {
		return s, err
	}
	s.IndexedCount = len(idx.Entries)

	return s, nil
}
