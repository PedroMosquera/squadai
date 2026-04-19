package doctor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/marker"
)

const catDrift = "Config Drift"

// runConfigDrift checks managed files for marker integrity and content drift.
func (d *Doctor) runConfigDrift(_ context.Context) []CheckResult {
	files, err := managed.ListManagedFiles(d.projectDir)
	if err != nil {
		return []CheckResult{warn(catDrift, "managed.json",
			fmt.Sprintf("could not read managed.json sidecar: %v", err), "", "")}
	}

	if len(files) == 0 {
		return []CheckResult{skip(catDrift, "managed files",
			"no managed files recorded yet (run 'squadai apply' first)")}
	}

	var results []CheckResult
	for _, relFile := range files {
		results = append(results, d.checkDriftFile(relFile))
	}
	return results
}

// checkDriftFile inspects a single managed file for integrity. JSON files
// (e.g. opencode.json, .mcp.json, settings.json) are validated by checking
// that all managed JSON keys are present in the parsed object — JSON has no
// comment syntax, so HTML markers cannot be used. All other files use the
// HTML marker convention (<!-- squadai:section --> ... <!-- /squadai:section -->).
func (d *Doctor) checkDriftFile(relFile string) CheckResult {
	absPath := filepath.Join(d.projectDir, relFile)
	name := filepath.Base(relFile)

	// Check if file exists.
	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fail(catDrift, name,
				fmt.Sprintf("%s — file deleted (was managed by squadai)", relFile),
				relFile,
				"Run 'squadai apply' to re-create it")
		}
		return warn(catDrift, name,
			fmt.Sprintf("%s — could not read file: %v", relFile, err), relFile, "")
	}

	// Read managed keys to find which sections we own.
	managedKeys, err := managed.ReadManagedKeys(d.projectDir, relFile)
	if err != nil {
		return warn(catDrift, name,
			fmt.Sprintf("%s — could not read managed keys: %v", relFile, err), relFile, "")
	}

	if isJSONFile(relFile) {
		return checkDriftJSON(relFile, name, data, managedKeys)
	}
	return checkDriftMarkers(relFile, name, string(data), managedKeys)
}

// isJSONFile returns true for paths whose extension is .json. JSON files
// cannot carry HTML comments, so they use a different drift-check strategy.
func isJSONFile(relFile string) bool {
	return strings.EqualFold(filepath.Ext(relFile), ".json")
}

// checkDriftJSON verifies that all managed JSON keys are present at the top
// level of the parsed JSON object. A missing key indicates the user removed
// (or never re-applied) a managed section.
func checkDriftJSON(relFile, name string, data []byte, managedKeys []string) CheckResult {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err != nil {
		return fail(catDrift, name,
			fmt.Sprintf("%s — invalid JSON: %v", relFile, err),
			relFile,
			"Restore from a backup or run 'squadai apply' to regenerate")
	}

	var missing []string
	for _, key := range managedKeys {
		if _, ok := obj[key]; !ok {
			missing = append(missing, key)
		}
	}

	if len(missing) > 0 {
		return fail(catDrift, name,
			fmt.Sprintf("%s — managed key(s) missing: %s (was it manually edited?)",
				relFile, strings.Join(missing, ", ")),
			relFile,
			"Run 'squadai apply' to restore the missing keys")
	}

	// Detect non-managed top-level keys as user content (informational, not a failure).
	for k := range obj {
		if !contains(managedKeys, k) {
			return CheckResult{
				Category: catDrift,
				Name:     name,
				Status:   CheckPass,
				Message:  fmt.Sprintf("%s — managed keys present, user keys detected outside managed scope", relFile),
				Detail:   relFile,
			}
		}
	}

	return pass(catDrift, name,
		fmt.Sprintf("%s — matches last apply", relFile), relFile)
}

// checkDriftMarkers verifies that every managed section's open and close HTML
// markers are present in a non-JSON file (markdown, text, etc.).
func checkDriftMarkers(relFile, name, content string, managedKeys []string) CheckResult {
	missingMarkers := []string{}
	for _, sectionID := range managedKeys {
		open := marker.OpenTag(sectionID)
		close := marker.CloseTag(sectionID)
		hasOpen := strings.Contains(content, open)
		hasClose := strings.Contains(content, close)
		if !hasOpen || !hasClose {
			missingMarkers = append(missingMarkers, sectionID)
		}
	}

	if len(missingMarkers) > 0 {
		return fail(catDrift, name,
			fmt.Sprintf("%s — marker block(s) missing: %s (was it manually edited?)",
				relFile, strings.Join(missingMarkers, ", ")),
			relFile,
			"Run 'squadai apply' to restore marker blocks")
	}

	if detectUserContent(content, managedKeys) {
		return CheckResult{
			Category: catDrift,
			Name:     name,
			Status:   CheckPass,
			Message:  fmt.Sprintf("%s — marker block intact, user content detected outside markers", relFile),
			Detail:   relFile,
		}
	}

	return pass(catDrift, name,
		fmt.Sprintf("%s — matches last apply", relFile), relFile)
}

// contains reports whether s is present in slice.
func contains(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// detectUserContent returns true when there is non-empty content outside all
// managed marker sections. This is a heuristic — any non-whitespace line that
// isn't inside a marker block counts as user content.
func detectUserContent(content string, managedKeys []string) bool {
	if len(managedKeys) == 0 {
		return strings.TrimSpace(content) != ""
	}

	lines := strings.Split(content, "\n")
	inManaged := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Check if we're entering or leaving a managed block.
		isOpen := false
		isClose := false
		for _, sectionID := range managedKeys {
			if strings.Contains(line, marker.OpenTag(sectionID)) {
				isOpen = true
			}
			if strings.Contains(line, marker.CloseTag(sectionID)) {
				isClose = true
			}
		}
		if isOpen {
			inManaged = true
			continue
		}
		if isClose {
			inManaged = false
			continue
		}
		if !inManaged {
			return true
		}
	}
	return false
}
