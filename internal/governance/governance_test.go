package governance_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/PedroMosquera/squadai/internal/governance"
	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/marker"
)

// ─── CheckDrift ──────────────────────────────────────────────────────────────

func TestCheckDrift_Empty_NoResults(t *testing.T) {
	dir := t.TempDir()
	results, err := governance.CheckDrift(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestCheckDrift_MarkerFile_Intact(t *testing.T) {
	dir := t.TempDir()
	relPath := "CLAUDE.md"
	content := marker.OpenTag("roles") + "\n# Roles\n" + marker.CloseTag("roles") + "\n"
	writeFile(t, dir, relPath, content)
	mustWriteManagedKeys(t, dir, relPath, []string{"roles"})

	results, err := governance.CheckDrift(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Drifted() {
		t.Errorf("expected intact, got kind=%q detail=%q", results[0].Kind, results[0].Detail)
	}
}

func TestCheckDrift_MarkerFile_Stripped(t *testing.T) {
	dir := t.TempDir()
	relPath := "CLAUDE.md"
	writeFile(t, dir, relPath, "# No markers here\n")
	mustWriteManagedKeys(t, dir, relPath, []string{"roles"})

	results, err := governance.CheckDrift(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Kind != governance.KindDriftMarkers {
		t.Errorf("expected KindDriftMarkers, got %+v", results)
	}
}

func TestCheckDrift_MultipleMarkerKeys_PartialStrip(t *testing.T) {
	dir := t.TempDir()
	relPath := "AGENTS.md"
	// Only "memory" markers are present; "roles" markers are stripped.
	content := marker.OpenTag("memory") + "\n# Memory\n" + marker.CloseTag("memory") + "\n"
	writeFile(t, dir, relPath, content)
	mustWriteManagedKeys(t, dir, relPath, []string{"memory", "roles"})

	results, err := governance.CheckDrift(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Kind != governance.KindDriftMarkers {
		t.Errorf("expected KindDriftMarkers, got %+v", results)
	}
}

func TestCheckDrift_FileDeleted(t *testing.T) {
	dir := t.TempDir()
	relPath := "AGENTS.md"
	mustWriteManagedKeys(t, dir, relPath, []string{"roles"})

	results, err := governance.CheckDrift(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Kind != governance.KindDriftDeleted {
		t.Errorf("expected KindDriftDeleted, got %+v", results)
	}
}

func TestCheckDrift_JSONFile_Intact(t *testing.T) {
	dir := t.TempDir()
	relPath := filepath.Join(".cursor", "mcp.json")
	writeJSON(t, dir, relPath, map[string]any{"mcpServers": map[string]any{}})
	mustWriteManagedKeys(t, dir, relPath, []string{"mcpServers"})

	results, err := governance.CheckDrift(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Drifted() {
		t.Errorf("expected intact JSON file, got %+v", results)
	}
}

func TestCheckDrift_JSONFile_KeyMissing(t *testing.T) {
	dir := t.TempDir()
	relPath := filepath.Join(".cursor", "mcp.json")
	writeJSON(t, dir, relPath, map[string]any{"other": 1})
	mustWriteManagedKeys(t, dir, relPath, []string{"mcpServers"})

	results, err := governance.CheckDrift(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Kind != governance.KindDriftJSONKeys {
		t.Errorf("expected KindDriftJSONKeys, got %+v", results)
	}
}

func TestCheckDrift_JSONFile_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	relPath := filepath.Join(".vscode", "settings.json")
	writeFile(t, dir, relPath, "{not json}")
	mustWriteManagedKeys(t, dir, relPath, []string{"mcp.servers"})

	results, err := governance.CheckDrift(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Kind != governance.KindDriftJSONKeys {
		t.Errorf("expected KindDriftJSONKeys for invalid JSON, got %+v", results)
	}
}

func TestCheckDrift_CreatedFile_Present(t *testing.T) {
	dir := t.TempDir()
	relPath := filepath.Join(".windsurf", "mcp_config.json")
	writeJSON(t, dir, relPath, map[string]any{})
	mustTrackCreated(t, dir, relPath)

	results, err := governance.CheckDrift(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Drifted() {
		t.Errorf("expected intact created file, got %+v", results)
	}
}

func TestCheckDrift_CreatedFile_Deleted(t *testing.T) {
	dir := t.TempDir()
	relPath := filepath.Join(".windsurf", "mcp_config.json")
	mustTrackCreated(t, dir, relPath)

	results, err := governance.CheckDrift(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Kind != governance.KindDriftDeleted {
		t.Errorf("expected KindDriftDeleted for missing created file, got %+v", results)
	}
}

// ─── AuditLog ────────────────────────────────────────────────────────────────

func TestAuditLog_AppendAndRead(t *testing.T) {
	dir := t.TempDir()
	log := governance.OpenAuditLog(dir)

	e := governance.Event{Kind: governance.KindDriftDeleted, Path: "CLAUDE.md", Detail: "gone"}
	if err := log.Append(e); err != nil {
		t.Fatal(err)
	}

	events, err := log.Read(0, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Kind != governance.KindDriftDeleted {
		t.Errorf("unexpected kind: %q", events[0].Kind)
	}
}

func TestAuditLog_FilterByKindPrefix(t *testing.T) {
	dir := t.TempDir()
	log := governance.OpenAuditLog(dir)

	_ = log.Append(governance.Event{Kind: governance.KindDriftDeleted, Path: "a"})
	_ = log.Append(governance.Event{Kind: governance.KindApplyStart})
	_ = log.Append(governance.Event{Kind: governance.KindDriftMarkers, Path: "b"})

	events, err := log.Read(0, "drift")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 drift events, got %d", len(events))
	}
}

func TestAuditLog_FilterBySince(t *testing.T) {
	dir := t.TempDir()
	log := governance.OpenAuditLog(dir)

	old := governance.Event{
		Kind:      governance.KindDriftDeleted,
		Path:      "old",
		Timestamp: time.Now().Add(-2 * time.Hour),
	}
	fresh := governance.Event{Kind: governance.KindApplyComplete, Timestamp: time.Now()}

	_ = log.Append(old)
	_ = log.Append(fresh)

	events, err := log.Read(time.Hour, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != governance.KindApplyComplete {
		t.Errorf("expected only the fresh event, got %+v", events)
	}
}

func TestAuditLog_Empty_ReturnsNil(t *testing.T) {
	dir := t.TempDir()
	log := governance.OpenAuditLog(dir)

	events, err := log.Read(0, "")
	if err != nil {
		t.Fatal(err)
	}
	if events != nil {
		t.Errorf("expected nil slice for empty log, got %+v", events)
	}
}

func TestAuditLog_HasDriftSince_True(t *testing.T) {
	dir := t.TempDir()
	log := governance.OpenAuditLog(dir)
	_ = log.Append(governance.Event{Kind: governance.KindDriftDeleted, Path: "x"})

	has, err := log.HasDriftSince(0)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Error("expected HasDriftSince to return true")
	}
}

func TestAuditLog_HasDriftSince_False(t *testing.T) {
	dir := t.TempDir()
	log := governance.OpenAuditLog(dir)
	_ = log.Append(governance.Event{Kind: governance.KindApplyComplete})

	has, err := log.HasDriftSince(0)
	if err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("expected HasDriftSince to return false with only non-drift events")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func writeFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	abs := filepath.Join(dir, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func writeJSON(t *testing.T, dir, relPath string, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, relPath, string(data))
}

func mustWriteManagedKeys(t *testing.T, dir, relPath string, keys []string) {
	t.Helper()
	if err := managed.WriteManagedKeys(dir, relPath, keys); err != nil {
		t.Fatal(err)
	}
}

func mustTrackCreated(t *testing.T, dir, relPath string) {
	t.Helper()
	if err := managed.TrackCreatedFile(dir, relPath); err != nil {
		t.Fatal(err)
	}
}
