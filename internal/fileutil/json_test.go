package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadJSONFile_NotExist_ReturnsNilNil(t *testing.T) {
	got, err := ReadJSONFile(filepath.Join(t.TempDir(), "missing.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil map, got %v", got)
	}
}

func TestReadJSONFile_EmptyFile_ReturnsNilNil(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadJSONFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil map, got %v", got)
	}
}

func TestReadJSONFile_ValidJSON_ReturnsParsed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := []byte(`{"key": "value", "num": 42}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	got, err := ReadJSONFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got["key"] != "value" {
		t.Errorf(`got["key"] = %v, want "value"`, got["key"])
	}
}

func TestReadJSONFile_InvalidJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{not valid json`), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := ReadJSONFile(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}
