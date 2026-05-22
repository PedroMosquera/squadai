package memory

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReindex(t *testing.T) {
	dir := t.TempDir()

	// Create two .md files in docs/memory/.
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	note1 := "Decision: chose PostgreSQL over MySQL for reliability"
	note2 := "Learning: always write tests before implementation"

	if err := os.WriteFile(filepath.Join(memDir, "note1.md"), []byte(note1), 0644); err != nil {
		t.Fatalf("write note1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "note2.md"), []byte(note2), 0644); err != nil {
		t.Fatalf("write note2: %v", err)
	}

	// Create an _inbox file that should be skipped.
	inboxDir := filepath.Join(memDir, "_inbox")
	if err := os.MkdirAll(inboxDir, 0755); err != nil {
		t.Fatalf("mkdir inbox: %v", err)
	}
	if err := os.WriteFile(filepath.Join(inboxDir, "draft.md"), []byte("draft content"), 0644); err != nil {
		t.Fatalf("write draft: %v", err)
	}

	// Create a README.md that should be skipped.
	if err := os.WriteFile(filepath.Join(memDir, "README.md"), []byte("# README"), 0644); err != nil {
		t.Fatalf("write README: %v", err)
	}

	count, err := Reindex(dir)
	if err != nil {
		t.Fatalf("Reindex: %v", err)
	}

	if count != 2 {
		t.Errorf("Reindex count = %d, want 2", count)
	}

	// Verify index file was written.
	indexFile := filepath.Join(dir, ".squadai", "memory-index.json")
	data, err := os.ReadFile(indexFile)
	if err != nil {
		t.Fatalf("read index file: %v", err)
	}

	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatalf("unmarshal index: %v", err)
	}

	if len(idx.Entries) != 2 {
		t.Errorf("index entries = %d, want 2", len(idx.Entries))
	}
	if idx.Built == "" {
		t.Error("index built timestamp is empty")
	}
}

func TestLoadIndex_Missing(t *testing.T) {
	dir := t.TempDir()
	idx, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex on missing: %v", err)
	}
	if len(idx.Entries) != 0 {
		t.Errorf("expected 0 entries for missing index, got %d", len(idx.Entries))
	}
}
