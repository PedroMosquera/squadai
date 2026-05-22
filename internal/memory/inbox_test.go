package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddInbox(t *testing.T) {
	dir := t.TempDir()

	note := "Decision: chose X over Y because of performance"
	rel, err := AddInbox(dir, note)
	if err != nil {
		t.Fatalf("AddInbox: %v", err)
	}

	// Verify the relative path has the expected prefix.
	if !strings.HasPrefix(rel, "docs/memory/_inbox/") {
		t.Errorf("unexpected relative path %q, want prefix docs/memory/_inbox/", rel)
	}

	// Verify the file exists with correct content.
	fullPath := filepath.Join(dir, rel)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("read created file: %v", err)
	}
	if string(data) != note {
		t.Errorf("file content = %q, want %q", string(data), note)
	}

	// Verify filename ends with .md.
	if !strings.HasSuffix(rel, ".md") {
		t.Errorf("filename does not end with .md: %q", rel)
	}

	// Verify filename does not contain colons (filesystem safety).
	base := filepath.Base(rel)
	if strings.Contains(base, ":") {
		t.Errorf("filename contains colon: %q", base)
	}
}

func TestListInbox(t *testing.T) {
	dir := t.TempDir()

	// Empty inbox returns nil slice without error.
	paths, err := ListInbox(dir)
	if err != nil {
		t.Fatalf("ListInbox on empty: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths, got %d", len(paths))
	}

	// Add two notes.
	if _, err := AddInbox(dir, "note 1"); err != nil {
		t.Fatalf("AddInbox 1: %v", err)
	}
	if _, err := AddInbox(dir, "note 2"); err != nil {
		t.Fatalf("AddInbox 2: %v", err)
	}

	paths, err = ListInbox(dir)
	if err != nil {
		t.Fatalf("ListInbox: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("expected 2 paths, got %d", len(paths))
	}
}
