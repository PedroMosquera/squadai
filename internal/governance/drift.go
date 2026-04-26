package governance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// DriftResult is the state of a single managed file.
type DriftResult struct {
	Path   string    // relative path from project root
	Kind   EventKind // non-empty means drift detected
	Detail string    // human-readable description
}

// Drifted returns true when drift was detected for this file.
func (r DriftResult) Drifted() bool { return r.Kind != "" }

// CheckDrift inspects every file recorded in managed.json for the project rooted
// at projectDir. Returns one DriftResult per file; Kind == "" means intact.
func CheckDrift(projectDir string) ([]DriftResult, error) {
	files, err := managed.ListManagedFiles(projectDir)
	if err != nil {
		return nil, fmt.Errorf("drift check: %w", err)
	}

	created, err := managed.ListCreatedFiles(projectDir)
	if err != nil {
		return nil, fmt.Errorf("drift check: %w", err)
	}

	var results []DriftResult
	seen := make(map[string]bool)

	for _, relPath := range files {
		seen[relPath] = true
		results = append(results, checkManagedFile(projectDir, relPath))
	}

	// Created files that are not also in managed_files get an existence-only check.
	for _, relPath := range created {
		if seen[relPath] {
			continue
		}
		results = append(results, checkCreatedFile(projectDir, relPath))
	}

	return results, nil
}

func checkManagedFile(projectDir, relPath string) DriftResult {
	absPath := filepath.Join(projectDir, relPath)

	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return DriftResult{Path: relPath, Kind: KindDriftDeleted, Detail: "file deleted"}
		}
		return DriftResult{Path: relPath, Kind: KindDriftDeleted,
			Detail: fmt.Sprintf("unreadable: %v", err)}
	}

	keys, err := managed.ReadManagedKeys(projectDir, relPath)
	if err != nil || len(keys) == 0 {
		return DriftResult{Path: relPath} // nothing to validate
	}

	if isJSONPath(relPath) {
		return checkJSONKeys(relPath, data, keys)
	}
	return checkMarkerBlocks(relPath, string(data), keys)
}

func checkCreatedFile(projectDir, relPath string) DriftResult {
	if _, err := os.Stat(filepath.Join(projectDir, relPath)); os.IsNotExist(err) {
		return DriftResult{Path: relPath, Kind: KindDriftDeleted, Detail: "created file deleted"}
	}
	return DriftResult{Path: relPath}
}

func checkJSONKeys(relPath string, data []byte, keys []string) DriftResult {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return DriftResult{Path: relPath, Kind: KindDriftJSONKeys, Detail: "invalid JSON"}
	}

	var missing []string
	for _, k := range keys {
		if _, ok := obj[k]; !ok {
			missing = append(missing, k)
		}
	}

	if len(missing) > 0 {
		return DriftResult{
			Path:   relPath,
			Kind:   KindDriftJSONKeys,
			Detail: fmt.Sprintf("managed key(s) missing: %s", strings.Join(missing, ", ")),
		}
	}
	return DriftResult{Path: relPath}
}

func checkMarkerBlocks(relPath, content string, keys []string) DriftResult {
	var stripped []string
	for _, id := range keys {
		if !strings.Contains(content, marker.OpenTag(id)) ||
			!strings.Contains(content, marker.CloseTag(id)) {
			stripped = append(stripped, id)
		}
	}

	if len(stripped) > 0 {
		return DriftResult{
			Path:   relPath,
			Kind:   KindDriftMarkers,
			Detail: fmt.Sprintf("marker block(s) stripped: %s", strings.Join(stripped, ", ")),
		}
	}
	return DriftResult{Path: relPath}
}

func isJSONPath(relPath string) bool {
	return strings.EqualFold(filepath.Ext(relPath), ".json")
}
