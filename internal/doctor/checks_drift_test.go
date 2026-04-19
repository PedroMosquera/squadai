package doctor

import (
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/managed"
)

// ─── JSON files: managed-key drift detection ────────────────────────────────

func TestCheckDriftFile_JSON_AllKeysPresent_Pass(t *testing.T) {
	dir := t.TempDir()

	// Write a managed JSON file with the expected key.
	jsonContent := []byte(`{
  "mcp": {
    "context7": {"type": "local"}
  }
}`)
	writeManagedJSON(t, dir, "opencode.json", jsonContent, []string{"mcp"})

	d := &Doctor{projectDir: dir}
	got := d.checkDriftFile("opencode.json")

	if got.Status != CheckPass {
		t.Fatalf("expected Pass for JSON with managed key present, got %v: %s", got.Status, got.Message)
	}
}

func TestCheckDriftFile_JSON_MissingManagedKey_Fail(t *testing.T) {
	dir := t.TempDir()

	// JSON without the expected "mcp" key — simulates user removal.
	jsonContent := []byte(`{"other": "value"}`)
	writeManagedJSON(t, dir, "opencode.json", jsonContent, []string{"mcp"})

	d := &Doctor{projectDir: dir}
	got := d.checkDriftFile("opencode.json")

	if got.Status != CheckFail {
		t.Fatalf("expected Fail when managed key missing, got %v: %s", got.Status, got.Message)
	}
	if !contains([]string{got.Message}, got.Message) || got.Message == "" {
		t.Errorf("expected non-empty failure message")
	}
}

func TestCheckDriftFile_JSON_MultipleManagedKeys_AllPresent_Pass(t *testing.T) {
	dir := t.TempDir()

	jsonContent := []byte(`{
  "_squadai_permissions": "managed",
  "permissions": {"allow": []}
}`)
	writeManagedJSON(t, dir, ".claude/settings.json",
		jsonContent, []string{"_squadai_permissions", "permissions"})

	d := &Doctor{projectDir: dir}
	got := d.checkDriftFile(".claude/settings.json")

	if got.Status != CheckPass {
		t.Fatalf("expected Pass when all managed keys present, got %v: %s", got.Status, got.Message)
	}
}

func TestCheckDriftFile_JSON_PartialMissing_FailListsMissing(t *testing.T) {
	dir := t.TempDir()

	// Only one of two managed keys present.
	jsonContent := []byte(`{"_squadai_permissions": "managed"}`)
	writeManagedJSON(t, dir, ".claude/settings.json",
		jsonContent, []string{"_squadai_permissions", "permissions"})

	d := &Doctor{projectDir: dir}
	got := d.checkDriftFile(".claude/settings.json")

	if got.Status != CheckFail {
		t.Fatalf("expected Fail, got %v", got.Status)
	}
	// The failure message should mention the missing key, not the present one.
	if !containsSubstring(got.Message, "permissions") {
		t.Errorf("expected message to mention missing 'permissions' key, got: %s", got.Message)
	}
	if containsSubstring(got.Message, "missing: _squadai_permissions") {
		t.Errorf("expected only 'permissions' to be listed missing, got: %s", got.Message)
	}
}

func TestCheckDriftFile_JSON_UserKeysOutsideManagedScope_Pass(t *testing.T) {
	dir := t.TempDir()

	// Managed key present + user added their own top-level key.
	jsonContent := []byte(`{
  "mcp": {"context7": {"type": "local"}},
  "user_setting": "preserved"
}`)
	writeManagedJSON(t, dir, "opencode.json", jsonContent, []string{"mcp"})

	d := &Doctor{projectDir: dir}
	got := d.checkDriftFile("opencode.json")

	if got.Status != CheckPass {
		t.Fatalf("expected Pass when user keys exist alongside managed keys, got %v: %s", got.Status, got.Message)
	}
	if !containsSubstring(got.Message, "user keys") {
		t.Errorf("expected message to note user keys, got: %s", got.Message)
	}
}

func TestCheckDriftFile_JSON_InvalidJSON_Fail(t *testing.T) {
	dir := t.TempDir()

	writeManagedJSON(t, dir, "opencode.json", []byte("not valid json"), []string{"mcp"})

	d := &Doctor{projectDir: dir}
	got := d.checkDriftFile("opencode.json")

	if got.Status != CheckFail {
		t.Fatalf("expected Fail on invalid JSON, got %v", got.Status)
	}
	if !containsSubstring(got.Message, "invalid JSON") {
		t.Errorf("expected message to mention invalid JSON, got: %s", got.Message)
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func writeManagedJSON(t *testing.T, projectDir, relPath string, content []byte, managedKeys []string) {
	t.Helper()
	abs := filepath.Join(projectDir, relPath)
	if _, err := fileutil.WriteAtomic(abs, content, 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
	if err := managed.WriteManagedKeys(projectDir, relPath, managedKeys); err != nil {
		t.Fatalf("write managed keys for %s: %v", relPath, err)
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
