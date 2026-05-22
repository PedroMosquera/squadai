package integration_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/cli"
	internalmemory "github.com/PedroMosquera/squadai/internal/memory"
)

// TestProjectMemoryE2E exercises the end-to-end project memory workflow:
//  1. RunInit creates the docs/memory/ scaffold
//  2. RunMemoryAdd saves a note to the inbox
//  3. RunMemorySearch finds the note by keyword
//  4. RunMemoryReindex writes .squadai/memory-index.json
//  5. The index has at least one entry after a note is promoted-like (placed in decisions/)
func TestProjectMemoryE2E(t *testing.T) {
	dir := t.TempDir()

	// Change into the temp dir so that RunInit and memory operations operate on it.
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to temp dir: %v", err)
	}

	// ── Step 1: RunInit creates docs/memory/ scaffold ─────────────────────────

	var initOut bytes.Buffer
	if err := cli.RunInit([]string{}, &initOut); err != nil {
		t.Fatalf("RunInit: %v", err)
	}

	// Verify scaffold files were created.
	scaffoldFiles := []string{
		filepath.Join("docs", "memory", "README.md"),
		filepath.Join("docs", "memory", "_inbox", "README.md"),
		filepath.Join("docs", "memory", "decisions", "README.md"),
		filepath.Join("docs", "memory", "learnings", "README.md"),
		filepath.Join("docs", "memory", "incidents", "README.md"),
	}
	for _, rel := range scaffoldFiles {
		full := filepath.Join(dir, rel)
		if _, statErr := os.Stat(full); statErr != nil {
			t.Errorf("scaffold file missing: %s", rel)
		}
	}

	// ── Step 2: RunMemoryAdd saves a note ────────────────────────────────────

	note := "Decision: chose BubbleTea for TUI rendering"
	if err := cli.RunMemoryAdd([]string{note}); err != nil {
		t.Fatalf("RunMemoryAdd: %v", err)
	}

	// Verify inbox contains at least one .md file.
	inboxDir := filepath.Join(dir, "docs", "memory", "_inbox")
	entries, readErr := os.ReadDir(inboxDir)
	if readErr != nil {
		t.Fatalf("read inbox dir: %v", readErr)
	}
	var mdFiles []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".md" && e.Name() != "README.md" {
			mdFiles = append(mdFiles, e.Name())
		}
	}
	if len(mdFiles) == 0 {
		t.Fatal("inbox should have at least one .md note after RunMemoryAdd")
	}

	// Verify the file contains the note text.
	notePath := filepath.Join(inboxDir, mdFiles[0])
	noteData, readErr := os.ReadFile(notePath)
	if readErr != nil {
		t.Fatalf("read note file: %v", readErr)
	}
	if string(noteData) != note {
		t.Errorf("note content = %q, want %q", string(noteData), note)
	}

	// ── Step 3: RunMemorySearch finds the note ────────────────────────────────
	// First, place a promoted note (non-inbox) so Search can find it via the index.
	// Write a note directly to decisions/ to simulate a promoted note.
	decisionsDir := filepath.Join(dir, "docs", "memory", "decisions")
	promotedNote := "Decision: BubbleTea chosen for TUI"
	promotedPath := filepath.Join(decisionsDir, "001-bubbletea.md")
	if writeErr := os.WriteFile(promotedPath, []byte(promotedNote), 0644); writeErr != nil {
		t.Fatalf("write promoted note: %v", writeErr)
	}

	// Build the index so Search can work.
	if err := cli.RunMemoryReindex([]string{}); err != nil {
		t.Fatalf("RunMemoryReindex (pre-search): %v", err)
	}

	// Search for a word from the promoted note.
	results, searchErr := internalmemory.Search(dir, "BubbleTea")
	if searchErr != nil {
		t.Fatalf("memory.Search: %v", searchErr)
	}
	if len(results) == 0 {
		t.Error("search for 'BubbleTea' should return at least one result")
	}

	// ── Step 4: RunMemoryReindex writes .squadai/memory-index.json ───────────

	if err := cli.RunMemoryReindex([]string{}); err != nil {
		t.Fatalf("RunMemoryReindex: %v", err)
	}

	indexPath := filepath.Join(dir, ".squadai", "memory-index.json")
	if _, statErr := os.Stat(indexPath); statErr != nil {
		t.Fatalf("memory-index.json not found at %s", indexPath)
	}

	// ── Step 5: Index has at least one entry ─────────────────────────────────

	indexData, readErr := os.ReadFile(indexPath)
	if readErr != nil {
		t.Fatalf("read memory-index.json: %v", readErr)
	}

	var idx struct {
		Entries []json.RawMessage `json:"entries"`
	}
	if err := json.Unmarshal(indexData, &idx); err != nil {
		t.Fatalf("parse memory-index.json: %v", err)
	}
	if len(idx.Entries) == 0 {
		t.Error("memory index should have at least one entry after reindex with promoted note")
	}
}

// TestRunInit_WithoutMemory verifies --without-memory skips the scaffold.
func TestRunInit_WithoutMemory(t *testing.T) {
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	var buf bytes.Buffer
	if err := cli.RunInit([]string{"--without-memory"}, &buf); err != nil {
		t.Fatalf("RunInit --without-memory: %v", err)
	}

	// docs/memory/ should NOT be created.
	memoryDir := filepath.Join(dir, "docs", "memory")
	if _, statErr := os.Stat(memoryDir); statErr == nil {
		t.Error("docs/memory/ should not be created when --without-memory is passed")
	}
}

// TestRunInit_WithMemoryIdempotent verifies that running RunInit twice does not
// overwrite existing docs/memory/ content.
func TestRunInit_WithMemoryIdempotent(t *testing.T) {
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// First init — creates scaffold.
	var buf1 bytes.Buffer
	if err := cli.RunInit([]string{}, &buf1); err != nil {
		t.Fatalf("first RunInit: %v", err)
	}

	// Overwrite the main README with custom content.
	customContent := "# My Custom Memory Notes"
	readmePath := filepath.Join(dir, "docs", "memory", "README.md")
	if writeErr := os.WriteFile(readmePath, []byte(customContent), 0644); writeErr != nil {
		t.Fatalf("write custom readme: %v", writeErr)
	}

	// Second init — must not overwrite the custom README.
	var buf2 bytes.Buffer
	if err := cli.RunInit([]string{}, &buf2); err != nil {
		t.Fatalf("second RunInit: %v", err)
	}

	data, readErr := os.ReadFile(readmePath)
	if readErr != nil {
		t.Fatalf("read readme after second init: %v", readErr)
	}
	if string(data) != customContent {
		t.Errorf("README.md was overwritten on second init; got %q, want %q", string(data), customContent)
	}
}
