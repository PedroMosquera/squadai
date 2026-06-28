package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeNote(t *testing.T, dir, rel, content string) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatalf("mkdir for %s: %v", rel, err)
	}
	if err := os.WriteFile(full, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", rel, err)
	}
}

func setModTime(t *testing.T, dir, rel string, mt time.Time) {
	t.Helper()
	full := filepath.Join(dir, rel)
	if err := os.Chtimes(full, mt, mt); err != nil {
		t.Fatalf("chtimes %s: %v", rel, err)
	}
}

func TestBuildTFIDFIndex_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	// No docs/memory/ at all.
	idx, err := buildTFIDFIndex(dir)
	if err != nil {
		t.Fatalf("buildTFIDFIndex on empty: %v", err)
	}
	if idx == nil {
		t.Fatal("expected non-nil index, got nil")
	}
	if len(idx.entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(idx.entries))
	}
	if idx.totalDocs != 0 {
		t.Errorf("expected totalDocs 0, got %d", idx.totalDocs)
	}
}

func TestBuildTFIDFIndex_WithDocs(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeNote(t, dir, "docs/memory/auth.md", "Authentication auth login flow with JWT")
	writeNote(t, dir, "docs/memory/database.md", "Database PostgreSQL storage and query optimization")
	writeNote(t, dir, "docs/memory/deploy.md", "Deployment Kubernetes rolling updates and rollout")

	idx, err := buildTFIDFIndex(dir)
	if err != nil {
		t.Fatalf("buildTFIDFIndex: %v", err)
	}
	if len(idx.entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(idx.entries))
	}
	for _, e := range idx.entries {
		if len(e.vector) == 0 {
			t.Errorf("entry %q has empty vector", e.path)
		}
		if e.norm <= 0 {
			t.Errorf("entry %q has non-positive norm %f", e.path, e.norm)
		}
	}
}

func TestBuildTFIDFIndex_SkipsInbox(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeNote(t, dir, "docs/memory/note.md", "Real note about testing")
	writeNote(t, dir, "docs/memory/_inbox/draft.md", "Inbox draft note")

	idx, err := buildTFIDFIndex(dir)
	if err != nil {
		t.Fatalf("buildTFIDFIndex: %v", err)
	}
	if len(idx.entries) != 1 {
		t.Fatalf("expected 1 entry (inbox skipped), got %d", len(idx.entries))
	}
	if filepath.Base(idx.entries[0].path) != "note.md" {
		t.Errorf("expected note.md, got %q", idx.entries[0].path)
	}
}

func TestBuildTFIDFIndex_SkipsReadme(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeNote(t, dir, "docs/memory/README.md", "# Memory readme")
	writeNote(t, dir, "docs/memory/note.md", "Real note about deployments")

	idx, err := buildTFIDFIndex(dir)
	if err != nil {
		t.Fatalf("buildTFIDFIndex: %v", err)
	}
	if len(idx.entries) != 1 {
		t.Fatalf("expected 1 entry (README skipped), got %d", len(idx.entries))
	}
	if filepath.Base(idx.entries[0].path) != "note.md" {
		t.Errorf("expected note.md, got %q", idx.entries[0].path)
	}
}

func TestSearchTFIDF_RelevantResults(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeNote(t, dir, "docs/memory/auth.md", "Authentication auth login flow JWT session management")
	writeNote(t, dir, "docs/memory/database.md", "Database PostgreSQL storage query optimization indexing")
	writeNote(t, dir, "docs/memory/deploy.md", "Deployment Kubernetes rolling updates rollout strategy")

	idx, err := buildTFIDFIndex(dir)
	if err != nil {
		t.Fatalf("buildTFIDFIndex: %v", err)
	}

	results := searchTFIDF(idx, "auth")
	if len(results) == 0 {
		t.Fatal("expected at least 1 result, got none")
	}
	if filepath.Base(results[0].Path) != "auth.md" {
		t.Errorf("expected auth.md to rank first, got %q", results[0].Path)
	}
	if results[0].Score <= 0 {
		t.Errorf("top result score = %f, want > 0", results[0].Score)
	}
}

func TestSearchTFIDF_EmptyQuery(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeNote(t, dir, "docs/memory/note.md", "Some note about Go testing")

	idx, err := buildTFIDFIndex(dir)
	if err != nil {
		t.Fatalf("buildTFIDFIndex: %v", err)
	}

	if results := searchTFIDF(idx, ""); results != nil {
		t.Errorf("expected nil results for empty query, got %v", results)
	}
}

func TestSearchTFIDF_NoIndex(t *testing.T) {
	dir := t.TempDir()
	// No docs at all.
	idx, err := buildTFIDFIndex(dir)
	if err != nil {
		t.Fatalf("buildTFIDFIndex: %v", err)
	}
	if results := searchTFIDF(idx, "anything"); results != nil {
		t.Errorf("expected nil results with no docs, got %v", results)
	}
}

func TestSearchTFIDF_FreshnessDecay(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := "shared learning note about testing patterns and practices"
	writeNote(t, dir, "docs/memory/doc_new.md", content)
	writeNote(t, dir, "docs/memory/doc_old.md", content)

	// Set mod times: one recent, one 200 days old.
	setModTime(t, dir, "docs/memory/doc_new.md", time.Now())
	setModTime(t, dir, "docs/memory/doc_old.md", time.Now().Add(-200*24*time.Hour))

	idx, err := buildTFIDFIndex(dir)
	if err != nil {
		t.Fatalf("buildTFIDFIndex: %v", err)
	}

	results := searchTFIDF(idx, "testing")
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if filepath.Base(results[0].Path) != "doc_new.md" {
		t.Errorf("expected doc_new.md to rank first, got %q", results[0].Path)
	}
	if results[0].Score <= results[1].Score {
		t.Errorf("expected newer doc to score higher: new=%f old=%f",
			results[0].Score, results[1].Score)
	}
}

func TestReindexTFIDF_WritesFile(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeNote(t, dir, "docs/memory/note.md", "Note about authentication")

	n, err := ReindexTFIDF(dir)
	if err != nil {
		t.Fatalf("ReindexTFIDF: %v", err)
	}
	if n != 1 {
		t.Errorf("expected count 1, got %d", n)
	}

	filePath := filepath.Join(dir, ".squadai", "memory-tfidf.json")
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("tfidf index file not created: %v", err)
	}
}

func TestReindexTFIDF_ReturnsCount(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeNote(t, dir, "docs/memory/a.md", "Alpha note about alpha")
	writeNote(t, dir, "docs/memory/b.md", "Beta note about beta")

	n, err := ReindexTFIDF(dir)
	if err != nil {
		t.Fatalf("ReindexTFIDF: %v", err)
	}
	if n != 2 {
		t.Errorf("expected count 2, got %d", n)
	}
}

func TestLoadTFIDFIndex_NoFile(t *testing.T) {
	dir := t.TempDir()
	idx, err := loadTFIDFIndex(dir)
	if err != nil {
		t.Fatalf("loadTFIDFIndex on missing: %v", err)
	}
	if idx != nil {
		t.Errorf("expected nil index for missing file, got %+v", idx)
	}
}

func TestLoadTFIDFIndex_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Use 3 docs so unique terms get a non-zero IDF (with N=2, a term in 1
	// of 2 docs yields idf = log(2/2) = 0).
	writeNote(t, dir, "docs/memory/one.md", "First note about caching strategies")
	writeNote(t, dir, "docs/memory/two.md", "Second note about logging pipelines")
	writeNote(t, dir, "docs/memory/three.md", "Third note about deployment patterns")

	fresh, err := buildTFIDFIndex(dir)
	if err != nil {
		t.Fatalf("buildTFIDFIndex: %v", err)
	}

	if _, err := ReindexTFIDF(dir); err != nil {
		t.Fatalf("ReindexTFIDF: %v", err)
	}

	loaded, err := loadTFIDFIndex(dir)
	if err != nil {
		t.Fatalf("loadTFIDFIndex: %v", err)
	}
	if loaded == nil {
		t.Fatal("expected non-nil index after reindex")
	}
	if len(loaded.entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(loaded.entries))
	}

	paths := map[string]bool{}
	for _, e := range loaded.entries {
		paths[filepath.Base(e.path)] = true
		if e.norm <= 0 {
			t.Errorf("entry %q has non-positive norm %f", e.path, e.norm)
		}
	}
	for _, want := range []string{"one.md", "two.md", "three.md"} {
		if !paths[want] {
			t.Errorf("round-trip missing %q in %+v", want, paths)
		}
	}

	// A loaded index should produce the same ranking as a freshly built one.
	want := searchTFIDF(fresh, "caching")
	got := searchTFIDF(loaded, "caching")
	if len(want) == 0 {
		t.Fatal("expected fresh index to return results for 'caching'")
	}
	if len(got) != len(want) {
		t.Fatalf("loaded result count = %d, want %d", len(got), len(want))
	}
	if got[0].Path != want[0].Path {
		t.Errorf("loaded top result = %q, want %q", got[0].Path, want[0].Path)
	}
	if filepath.Base(got[0].Path) != "one.md" {
		t.Errorf("expected one.md to rank first, got %q", got[0].Path)
	}
}
