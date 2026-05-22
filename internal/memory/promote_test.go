package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPromote(t *testing.T) {
	dir := t.TempDir()

	note := "Incident: server outage at 14:00 UTC"
	rel, err := AddInbox(dir, note)
	if err != nil {
		t.Fatalf("AddInbox: %v", err)
	}

	// Promote the note to "incidents" category.
	newRel, err := Promote(dir, rel, "incidents")
	if err != nil {
		t.Fatalf("Promote: %v", err)
	}

	// Source file should be deleted.
	srcPath := filepath.Join(dir, rel)
	if _, statErr := os.Stat(srcPath); !os.IsNotExist(statErr) {
		t.Error("source file still exists after promote")
	}

	// Destination file should exist.
	destPath := filepath.Join(dir, newRel)
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read promoted file: %v", err)
	}

	content := string(data)

	// Should contain frontmatter.
	if !strings.HasPrefix(content, "---\n") {
		t.Errorf("promoted file does not start with frontmatter, got: %q", content[:min(50, len(content))])
	}
	if !strings.Contains(content, "category: incidents") {
		t.Errorf("promoted file does not contain category frontmatter: %q", content)
	}
	if !strings.Contains(content, "date:") {
		t.Errorf("promoted file does not contain date frontmatter: %q", content)
	}

	// Should contain original note.
	if !strings.Contains(content, note) {
		t.Errorf("promoted file does not contain original note: %q", content)
	}

	// Destination path should be under docs/memory/incidents/.
	if !strings.Contains(newRel, "incidents") {
		t.Errorf("promoted path %q does not contain category", newRel)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
