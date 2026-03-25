package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

// ─── WriteAtomic tests ──────────────────────────────────────────────────────

func TestWriteAtomic_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "new.txt")

	res, err := WriteAtomic(path, []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Changed {
		t.Error("expected Changed=true for new file")
	}
	if !res.Created {
		t.Error("expected Created=true for new file")
	}

	got, _ := os.ReadFile(path)
	if string(got) != "hello" {
		t.Errorf("content = %q, want %q", got, "hello")
	}
}

func TestWriteAtomic_IdempotentWhenUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "idem.txt")

	WriteAtomic(path, []byte("same"), 0644)

	res, err := WriteAtomic(path, []byte("same"), 0644)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Changed {
		t.Error("expected Changed=false when content is identical")
	}
	if res.Created {
		t.Error("expected Created=false on second write")
	}
}

func TestWriteAtomic_UpdatesWhenDifferent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "update.txt")

	WriteAtomic(path, []byte("old"), 0644)

	res, err := WriteAtomic(path, []byte("new"), 0644)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Changed {
		t.Error("expected Changed=true when content differs")
	}
	if res.Created {
		t.Error("expected Created=false on update")
	}

	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Errorf("content = %q, want %q", got, "new")
	}
}

func TestWriteAtomic_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "deep", "nested", "file.txt")

	res, err := WriteAtomic(path, []byte("deep"), 0644)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Created {
		t.Error("expected Created=true")
	}
}

func TestWriteAtomic_NoTempFileLeftOnSuccess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clean.txt")

	WriteAtomic(path, []byte("data"), 0644)

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "clean.txt" {
			t.Errorf("unexpected file in dir: %s", e.Name())
		}
	}
}

// ─── ReadFileOrEmpty tests ──────────────────────────────────────────────────

func TestReadFileOrEmpty_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.txt")
	os.WriteFile(path, []byte("content"), 0644)

	data, err := ReadFileOrEmpty(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("data = %q, want %q", data, "content")
	}
}

func TestReadFileOrEmpty_MissingFile(t *testing.T) {
	data, err := ReadFileOrEmpty("/nonexistent/path/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected empty bytes for missing file, got %d bytes", len(data))
	}
}
