package managed

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/marker"
)

// StaleContent is the result of removing SquadAI's content from one file in
// memory. Nothing is written.
type StaleContent struct {
	// Stripped is the file content with SquadAI's content removed.
	Stripped []byte
	// Found reports whether any SquadAI content was recognized and removed.
	Found bool
	// OnlySquadAI reports whether nothing but whitespace, or an empty JSON
	// object, remains after removal.
	OnlySquadAI bool
	// Created reports whether the sidecar records SquadAI creating the file.
	Created bool
}

// Deletable reports whether the whole file may be removed. Creation alone is
// not enough (the user may have added content since), and neither is an empty
// remainder (the user may own a file that only holds SquadAI content, or an
// install predating created-file tracking). Otherwise strip in place.
func (s StaleContent) Deletable() bool {
	return s.Found && s.OnlySquadAI && s.Created
}

// InspectStale computes what removing SquadAI's content from the file at path
// would leave. Marker blocks (HTML or hash comments) are stripped from any
// file; for .json files the top-level keys recorded in the sidecar are
// removed instead. A JSON file that does not parse as an object reports
// Found=false, so callers keep it. exists is false when path is absent.
func InspectStale(projectRoot, path string) (stale StaleContent, exists bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return StaleContent{}, false, nil
		}
		return StaleContent{}, false, fmt.Errorf("read %s: %w", path, err)
	}

	mu.Lock()
	doc, err := readSidecar(projectRoot)
	mu.Unlock()
	if err != nil {
		return StaleContent{}, true, err
	}
	rel := relPath(projectRoot, path)
	for _, f := range doc.CreatedFiles {
		if f == rel {
			stale.Created = true
			break
		}
	}

	if strings.EqualFold(filepath.Ext(path), ".json") {
		entry := doc.ManagedFiles[rel]
		stale.Stripped, stale.Found, stale.OnlySquadAI = stripOwnedJSON(data, entry.ManagedKeys, entry.ManagedEntries)
		return stale, true, nil
	}

	stripped, _ := marker.StripAll(string(data))
	stale.Stripped = []byte(stripped)
	stale.Found = stripped != string(data)
	stale.OnlySquadAI = strings.TrimSpace(stripped) == ""
	return stale, true, nil
}

// stripOwnedJSON removes SquadAI's content from a JSON object: the named
// children for keys with per-entry ownership (MCP servers), otherwise the
// whole owned top-level key. This is the single place that interprets the
// sidecar for removal. Output matches MergeAndWriteJSON's format.
func stripOwnedJSON(data []byte, owned []string, entries map[string][]string) (out []byte, found, empty bool) {
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil || obj == nil {
		return data, false, false
	}
	for _, key := range owned {
		val, ok := obj[key]
		if !ok {
			continue
		}
		names, perEntry := entries[key]
		if !perEntry {
			delete(obj, key)
			found = true
			continue
		}
		children, isObj := val.(map[string]any)
		if !isObj {
			continue
		}
		for _, name := range names {
			if _, ok := children[name]; ok {
				delete(children, name)
				found = true
			}
		}
		if len(children) == 0 {
			delete(obj, key)
		}
	}
	if !found {
		return data, false, false
	}
	out, err := json.MarshalIndent(obj, "", "  ")
	if err != nil {
		return data, false, false
	}
	return append(out, '\n'), true, len(obj) == 0
}

// ForgetFile drops every sidecar record for the file at path (created-file
// entry and managed keys). path may be absolute; it is resolved the same way
// the installers resolve it when recording.
func ForgetFile(projectRoot, path string) error {
	mu.Lock()
	defer mu.Unlock()

	doc, err := readSidecar(projectRoot)
	if err != nil {
		return err
	}
	rel := relPath(projectRoot, path)

	changed := false
	if _, ok := doc.ManagedFiles[rel]; ok {
		delete(doc.ManagedFiles, rel)
		changed = true
	}
	kept := doc.CreatedFiles[:0]
	for _, f := range doc.CreatedFiles {
		if f == rel {
			changed = true
			continue
		}
		kept = append(kept, f)
	}
	doc.CreatedFiles = kept
	if !changed {
		return nil
	}
	return writeSidecar(projectRoot, doc)
}

func relPath(projectRoot, path string) string {
	if projectRoot == "" || !filepath.IsAbs(path) {
		return path
	}
	if rel, err := filepath.Rel(projectRoot, path); err == nil {
		return rel
	}
	return path
}
