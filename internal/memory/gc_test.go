package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGC_NoMemoryDir(t *testing.T) {
	dir := t.TempDir()
	// No docs/memory/ directory at all.
	res, err := GC(dir, 30*24*time.Hour, false)
	if err != nil {
		t.Fatalf("GC on missing memory dir: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
	if len(res.Archived) != 0 {
		t.Errorf("expected 0 archived, got %d", len(res.Archived))
	}
	if res.Remaining != 0 {
		t.Errorf("expected 0 remaining, got %d", res.Remaining)
	}
}

func TestGC_NothingStale(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeNote(t, dir, "docs/memory/recent.md", "Recent note about caching")

	res, err := GC(dir, 90*24*time.Hour, false)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if len(res.Archived) != 0 {
		t.Errorf("expected 0 archived, got %d: %v", len(res.Archived), res.Archived)
	}
	if _, err := os.Stat(filepath.Join(memDir, "recent.md")); err != nil {
		t.Errorf("recent note should still exist: %v", err)
	}
}

func TestGC_ArchivesStale(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeNote(t, dir, "docs/memory/stale.md", "Old note about legacy auth")
	setModTime(t, dir, "docs/memory/stale.md", time.Now().Add(-200*24*time.Hour))

	res, err := GC(dir, 90*24*time.Hour, false)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if len(res.Archived) != 1 {
		t.Fatalf("expected 1 archived, got %d: %v", len(res.Archived), res.Archived)
	}
	// Original file gone.
	if _, err := os.Stat(filepath.Join(memDir, "stale.md")); !os.IsNotExist(err) {
		t.Errorf("expected stale.md to be moved, stat err: %v", err)
	}
	// Archived copy exists.
	archived := filepath.Join(memDir, ".archive", "stale.md")
	if _, err := os.Stat(archived); err != nil {
		t.Errorf("expected archived copy at %s: %v", archived, err)
	}
}

func TestGC_ExemptsReferenced(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeNote(t, dir, "docs/memory/old.md", "Old note about legacy decisions")
	setModTime(t, dir, "docs/memory/old.md", time.Now().Add(-200*24*time.Hour))

	// ADR referencing old.md by filename.
	writeNote(t, dir, "docs/memory/decisions/adr-0001.md", "ADR: superseded by old.md guidance")

	res, err := GC(dir, 90*24*time.Hour, false)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if len(res.Archived) != 0 {
		t.Errorf("expected 0 archived (referenced note exempt), got %d: %v",
			len(res.Archived), res.Archived)
	}
	if _, err := os.Stat(filepath.Join(memDir, "old.md")); err != nil {
		t.Errorf("referenced old.md should still exist: %v", err)
	}
}

func TestGC_DryRun(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeNote(t, dir, "docs/memory/stale.md", "Old note about legacy caching")
	setModTime(t, dir, "docs/memory/stale.md", time.Now().Add(-200*24*time.Hour))

	res, err := GC(dir, 90*24*time.Hour, true)
	if err != nil {
		t.Fatalf("GC dry-run: %v", err)
	}
	if !res.DryRun {
		t.Error("expected DryRun=true")
	}
	if len(res.Archived) != 1 {
		t.Fatalf("expected 1 reported archived, got %d", len(res.Archived))
	}
	// File must NOT have been moved.
	if _, err := os.Stat(filepath.Join(memDir, "stale.md")); err != nil {
		t.Errorf("stale.md should still exist after dry-run: %v", err)
	}
	// No archive directory should have been created.
	if _, err := os.Stat(filepath.Join(memDir, ".archive")); !os.IsNotExist(err) {
		t.Errorf("archive dir should not exist after dry-run, stat err: %v", err)
	}
}

func TestGC_PreservesSubdirStructure(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeNote(t, dir, "docs/memory/learnings/sub/deep.md", "Deep learning note")
	setModTime(t, dir, "docs/memory/learnings/sub/deep.md", time.Now().Add(-200*24*time.Hour))

	res, err := GC(dir, 90*24*time.Hour, false)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if len(res.Archived) != 1 {
		t.Fatalf("expected 1 archived, got %d", len(res.Archived))
	}

	archived := filepath.Join(memDir, ".archive", "learnings", "sub", "deep.md")
	if _, err := os.Stat(archived); err != nil {
		t.Errorf("expected archived copy at %s: %v", archived, err)
	}
	if _, err := os.Stat(filepath.Join(memDir, "learnings", "sub", "deep.md")); !os.IsNotExist(err) {
		t.Errorf("original deep.md should be gone, stat err: %v", err)
	}
}

func TestGC_SkipsInbox(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeNote(t, dir, "docs/memory/_inbox/draft.md", "Inbox draft")
	setModTime(t, dir, "docs/memory/_inbox/draft.md", time.Now().Add(-200*24*time.Hour))

	res, err := GC(dir, 90*24*time.Hour, false)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if len(res.Archived) != 0 {
		t.Errorf("expected 0 archived (inbox skipped), got %d: %v",
			len(res.Archived), res.Archived)
	}
	if _, err := os.Stat(filepath.Join(memDir, "_inbox", "draft.md")); err != nil {
		t.Errorf("inbox draft should still exist: %v", err)
	}
}

func TestGC_SkipsArchive(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// A file already living in .archive/ should not be re-archived.
	writeNote(t, dir, "docs/memory/.archive/old.md", "Already archived note")
	setModTime(t, dir, "docs/memory/.archive/old.md", time.Now().Add(-400*24*time.Hour))

	res, err := GC(dir, 90*24*time.Hour, false)
	if err != nil {
		t.Fatalf("GC: %v", err)
	}
	if len(res.Archived) != 0 {
		t.Errorf("expected 0 archived (.archive skipped), got %d: %v",
			len(res.Archived), res.Archived)
	}
	if _, err := os.Stat(filepath.Join(memDir, ".archive", "old.md")); err != nil {
		t.Errorf("already-archived file should still exist: %v", err)
	}
}

func TestGC_UpdatesIndex(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "docs", "memory")
	if err := os.MkdirAll(memDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	writeNote(t, dir, "docs/memory/stale.md", "Old note about legacy auth")
	writeNote(t, dir, "docs/memory/fresh.md", "Fresh note about caching")
	setModTime(t, dir, "docs/memory/stale.md", time.Now().Add(-200*24*time.Hour))

	// Build the live index with both notes.
	if _, err := Reindex(dir); err != nil {
		t.Fatalf("Reindex: %v", err)
	}
	before, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex before: %v", err)
	}
	if len(before.Entries) != 2 {
		t.Fatalf("expected 2 indexed entries before GC, got %d", len(before.Entries))
	}

	if _, err := GC(dir, 90*24*time.Hour, false); err != nil {
		t.Fatalf("GC: %v", err)
	}

	after, err := LoadIndex(dir)
	if err != nil {
		t.Fatalf("LoadIndex after: %v", err)
	}
	if len(after.Entries) != 1 {
		t.Fatalf("expected 1 indexed entry after GC, got %d", len(after.Entries))
	}
	if filepath.Base(after.Entries[0].Path) != "fresh.md" {
		t.Errorf("expected fresh.md to remain in index, got %q", after.Entries[0].Path)
	}
	// Ensure no archived paths leak into the live index.
	for _, e := range after.Entries {
		if strings.Contains(filepath.ToSlash(e.Path), "/.archive/") {
			t.Errorf("archived path should not be in live index: %q", e.Path)
		}
	}
}
