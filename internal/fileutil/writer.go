package fileutil

import (
	"bytes"
	"os"
	"path/filepath"
)

// WriteResult reports what happened during a write.
type WriteResult struct {
	// Changed is true if the file content was modified (or created).
	Changed bool

	// Created is true if the file did not exist before.
	Created bool
}

// WriteAtomic writes content to path atomically using a temp file + rename.
// It creates parent directories as needed. If the file already exists with
// identical content, it is left untouched (idempotent).
func WriteAtomic(path string, content []byte, perm os.FileMode) (WriteResult, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return WriteResult{}, err
	}

	// Check existing content for idempotency.
	existing, readErr := os.ReadFile(path)
	created := os.IsNotExist(readErr)
	if readErr == nil && bytes.Equal(existing, content) {
		return WriteResult{Changed: false, Created: false}, nil
	}

	// Write to temp file in the same directory (ensures same filesystem for rename).
	tmp, err := os.CreateTemp(dir, ".squadai-*.tmp")
	if err != nil {
		return WriteResult{}, err
	}
	tmpName := tmp.Name()

	// Clean up temp file on any failure path.
	defer func() {
		if tmpName != "" {
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return WriteResult{}, err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return WriteResult{}, err
	}
	if err := tmp.Close(); err != nil {
		return WriteResult{}, err
	}

	// Atomic rename.
	if err := os.Rename(tmpName, path); err != nil {
		return WriteResult{}, err
	}
	tmpName = "" // prevent deferred cleanup

	return WriteResult{Changed: true, Created: created}, nil
}

// ReadFileOrEmpty reads a file, returning empty bytes if it doesn't exist.
func ReadFileOrEmpty(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []byte{}, nil
		}
		return nil, err
	}
	return data, nil
}
