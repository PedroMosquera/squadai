package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatus(t *testing.T) {
	dir := t.TempDir()

	// Add two inbox notes.
	if _, err := AddInbox(dir, "note A"); err != nil {
		t.Fatalf("AddInbox A: %v", err)
	}
	if _, err := AddInbox(dir, "note B"); err != nil {
		t.Fatalf("AddInbox B: %v", err)
	}

	// Add one promoted note in decisions/.
	decisionsDir := filepath.Join(dir, "docs", "memory", "decisions")
	if err := os.MkdirAll(decisionsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(decisionsDir, "arch.md"), []byte("use microservices"), 0644); err != nil {
		t.Fatalf("write arch.md: %v", err)
	}

	// Build index.
	if _, err := Reindex(dir); err != nil {
		t.Fatalf("Reindex: %v", err)
	}

	s, err := Status(dir)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}

	if s.InboxCount != 2 {
		t.Errorf("InboxCount = %d, want 2", s.InboxCount)
	}
	if s.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1 (only non-inbox .md files)", s.TotalCount)
	}
	if s.IndexedCount != 1 {
		t.Errorf("IndexedCount = %d, want 1", s.IndexedCount)
	}
}

func TestStatus_Empty(t *testing.T) {
	dir := t.TempDir()

	s, err := Status(dir)
	if err != nil {
		t.Fatalf("Status on empty: %v", err)
	}
	if s.InboxCount != 0 || s.TotalCount != 0 || s.IndexedCount != 0 {
		t.Errorf("expected all zeros for empty, got %+v", s)
	}
}
