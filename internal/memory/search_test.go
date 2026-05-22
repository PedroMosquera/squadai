package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSearch(t *testing.T) {
	dir := t.TempDir()

	// Set up memory directory with two notes.
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(memDir, "database.md"),
		[]byte("Decision: chose PostgreSQL for reliability and ACID compliance"), 0644); err != nil {
		t.Fatalf("write note: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "testing.md"),
		[]byte("Learning: always write tests before implementation in Go"), 0644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	// Reindex first.
	if _, err := Reindex(dir); err != nil {
		t.Fatalf("Reindex: %v", err)
	}

	// Search for a word that only appears in the first note.
	results, err := Search(dir, "postgresql")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Search returned no results, expected at least 1")
	}

	// The first result should have score > 0.
	if results[0].Score <= 0 {
		t.Errorf("first result score = %f, want > 0", results[0].Score)
	}

	// Should match database.md.
	found := false
	for _, r := range results {
		if filepath.Base(r.Path) == "database.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("database.md not found in results: %+v", results)
	}
}

func TestSearch_NoResults(t *testing.T) {
	dir := t.TempDir()

	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "note.md"),
		[]byte("Decision: use Go for performance"), 0644); err != nil {
		t.Fatalf("write note: %v", err)
	}

	if _, err := Reindex(dir); err != nil {
		t.Fatalf("Reindex: %v", err)
	}

	results, err := Search(dir, "python")
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for unmatched query, got %d", len(results))
	}
}

func TestSearch_MissingIndex(t *testing.T) {
	dir := t.TempDir()
	results, err := Search(dir, "anything")
	if err != nil {
		t.Fatalf("Search on missing index: %v", err)
	}
	if results != nil {
		t.Errorf("expected nil results for missing index, got %v", results)
	}
}
