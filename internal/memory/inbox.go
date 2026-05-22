package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// AddInbox appends a new note to docs/memory/_inbox/<timestamp>.md.
// Returns the relative path of the created file.
func AddInbox(projectDir, note string) (string, error) {
	inboxDir := filepath.Join(projectDir, "docs", "memory", "_inbox")
	if err := os.MkdirAll(inboxDir, 0755); err != nil {
		return "", err
	}

	// Build filename: RFC3339 UTC with colons replaced by hyphens.
	// Include nanoseconds to ensure uniqueness when notes are added in quick succession.
	now := time.Now().UTC()
	ts := now.Format(time.RFC3339)
	ts = strings.ReplaceAll(ts, ":", "-")
	nanos := fmt.Sprintf("%09d", now.Nanosecond())
	filename := ts + "-" + nanos + ".md"

	fullPath := filepath.Join(inboxDir, filename)
	if err := os.WriteFile(fullPath, []byte(note), 0644); err != nil {
		return "", err
	}

	rel, err := filepath.Rel(projectDir, fullPath)
	if err != nil {
		return fullPath, nil
	}
	return rel, nil
}

// ListInbox returns relative paths of all files in docs/memory/_inbox/.
func ListInbox(projectDir string) ([]string, error) {
	inboxDir := filepath.Join(projectDir, "docs", "memory", "_inbox")
	entries, err := os.ReadDir(inboxDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var paths []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		fullPath := filepath.Join(inboxDir, e.Name())
		rel, relErr := filepath.Rel(projectDir, fullPath)
		if relErr != nil {
			rel = fullPath
		}
		paths = append(paths, rel)
	}
	return paths, nil
}
