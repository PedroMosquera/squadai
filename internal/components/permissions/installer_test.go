package permissions

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/cursor"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/adapters/windsurf"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// ─── Interface compliance ───────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	inst := New()
	if inst.ID() != domain.ComponentPermissions {
		t.Errorf("ID() = %q, want %q", inst.ID(), domain.ComponentPermissions)
	}
}

// ─── Plan ───────────────────────────────────────────────────────────────────

func TestPlan_EmitsActionsForSupportedAdapters(t *testing.T) {
	tests := []struct {
		name       string
		adapter    domain.Adapter
		wantAction bool
	}{
		{"claude", claude.New(), true},
		{"opencode", opencode.New(), true},
		{"vscode", vscode.New(), true},
		{"cursor", cursor.New(), false},
		{"windsurf", windsurf.New(), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			projectDir := t.TempDir()
			inst := New()
			actions, err := inst.Plan(tc.adapter, "", projectDir)
			if err != nil {
				t.Fatalf("Plan() error: %v", err)
			}

			hasNonSkip := false
			for _, a := range actions {
				if a.Action != domain.ActionSkip {
					hasNonSkip = true
				}
			}

			if tc.wantAction && !hasNonSkip {
				t.Errorf("expected non-skip action for %s, got none", tc.name)
			}
			if !tc.wantAction && hasNonSkip {
				t.Errorf("expected no action for %s, got one", tc.name)
			}
		})
	}
}

func TestPlan_SkipsWhenAlreadyApplied(t *testing.T) {
	projectDir := t.TempDir()
	settingsDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-seed with marker present.
	existing := map[string]interface{}{
		"someUserKey": "value",
		metaKey:       metaValue,
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	inst := New()
	actions, err := inst.Plan(claude.New(), "", projectDir)
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}

	if len(actions) == 0 {
		t.Fatal("expected at least one action")
	}
	if actions[0].Action != domain.ActionSkip {
		t.Errorf("expected ActionSkip, got %s", actions[0].Action)
	}
}

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_ClaudeMergesIntoExistingSettings(t *testing.T) {
	projectDir := t.TempDir()
	settingsDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-seed with user keys.
	existing := map[string]interface{}{
		"model":      "claude-sonnet-4-5",
		"userCustom": true,
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	settingsFile := filepath.Join(settingsDir, "settings.json")
	if err := os.WriteFile(settingsFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	inst := New()
	inst.projectDir = projectDir
	action := domain.PlannedAction{
		ID:         "claude-permissions",
		Agent:      domain.AgentClaudeCode,
		Component:  domain.ComponentPermissions,
		Action:     domain.ActionUpdate,
		TargetPath: settingsFile,
	}

	if err := inst.Apply(action); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	result := readJSON(t, settingsFile)

	// User keys preserved.
	if result["model"] != "claude-sonnet-4-5" {
		t.Error("user 'model' key was overwritten")
	}
	if result["userCustom"] != true {
		t.Error("user 'userCustom' key was overwritten")
	}

	// Permissions block present.
	perms, ok := result["permissions"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'permissions' key with object value")
	}
	deny, _ := toStringSlice(perms["deny"])
	if len(deny) == 0 {
		t.Error("expected deny entries in permissions block")
	}
	ask, _ := toStringSlice(perms["ask"])
	if len(ask) == 0 {
		t.Error("expected ask entries in permissions block")
	}

	// Marker present.
	if result[metaKey] != metaValue {
		t.Error("expected meta marker key to be set")
	}
}

func TestApply_OpenCodeWritesPermissionStructure(t *testing.T) {
	projectDir := t.TempDir()
	settingsFile := filepath.Join(projectDir, "opencode.json")

	inst := New()
	inst.projectDir = projectDir
	action := domain.PlannedAction{
		Agent:      domain.AgentOpenCode,
		Component:  domain.ComponentPermissions,
		Action:     domain.ActionCreate,
		TargetPath: settingsFile,
	}

	if err := inst.Apply(action); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	result := readJSON(t, settingsFile)

	permission, ok := result["permission"].(map[string]interface{})
	if !ok {
		t.Fatal("expected 'permission' key with object value")
	}

	bash, ok := permission["bash"].(map[string]interface{})
	if !ok || len(bash) == 0 {
		t.Error("expected non-empty permission.bash")
	}

	read, ok := permission["read"].(map[string]interface{})
	if !ok || len(read) == 0 {
		t.Error("expected non-empty permission.read")
	}

	for _, cmd := range confirmCommands {
		if bash[cmd] != "ask" {
			t.Errorf("expected bash[%q] = ask", cmd)
		}
	}
	for _, path := range denyPaths {
		if read[path] != "deny" {
			t.Errorf("expected read[%q] = deny", path)
		}
	}
}

func TestApply_VSCodeMergesReadonlyInclude(t *testing.T) {
	projectDir := t.TempDir()
	vscodeDir := filepath.Join(projectDir, ".vscode")
	if err := os.MkdirAll(vscodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	settingsFile := filepath.Join(vscodeDir, "settings.json")

	inst := New()
	inst.projectDir = projectDir
	action := domain.PlannedAction{
		Agent:      domain.AgentVSCodeCopilot,
		Component:  domain.ComponentPermissions,
		Action:     domain.ActionCreate,
		TargetPath: settingsFile,
	}

	if err := inst.Apply(action); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	result := readJSON(t, settingsFile)

	if result["chat.tools.autoApprove"] != false {
		t.Error("expected chat.tools.autoApprove = false")
	}

	readonlyInclude, ok := result["files.readonlyInclude"].(map[string]interface{})
	if !ok || len(readonlyInclude) == 0 {
		t.Error("expected non-empty files.readonlyInclude")
	}
	for _, p := range denyPaths {
		if readonlyInclude[p] != true {
			t.Errorf("expected files.readonlyInclude[%q] = true", p)
		}
	}
}

func TestApply_DoesNotOverwriteUserPermissions(t *testing.T) {
	projectDir := t.TempDir()
	settingsDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// User already has a custom deny entry.
	existing := map[string]interface{}{
		"permissions": map[string]interface{}{
			"deny": []interface{}{"Read(./custom-secret)"},
			"ask":  []interface{}{"Bash(custom-danger*)"},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	settingsFile := filepath.Join(settingsDir, "settings.json")
	if err := os.WriteFile(settingsFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	inst := New()
	inst.projectDir = projectDir
	action := domain.PlannedAction{
		Agent:      domain.AgentClaudeCode,
		Component:  domain.ComponentPermissions,
		Action:     domain.ActionUpdate,
		TargetPath: settingsFile,
	}

	if err := inst.Apply(action); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	result := readJSON(t, settingsFile)
	perms, ok := result["permissions"].(map[string]interface{})
	if !ok {
		t.Fatal("expected permissions block")
	}

	deny, _ := toStringSlice(perms["deny"])
	hasUser := false
	for _, d := range deny {
		if d == "Read(./custom-secret)" {
			hasUser = true
		}
	}
	if !hasUser {
		t.Error("user deny entry 'Read(./custom-secret)' was removed")
	}

	// Also has our managed entries.
	hasManagedEntry := false
	for _, d := range deny {
		if d == "Read(./.env*)" {
			hasManagedEntry = true
		}
	}
	if !hasManagedEntry {
		t.Error("managed deny entry 'Read(./.env*)' is missing")
	}
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestVerify_DetectsManaged(t *testing.T) {
	projectDir := t.TempDir()
	settingsDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// File with marker present.
	existing := map[string]interface{}{metaKey: metaValue}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	results, err := New().Verify(claude.New(), "", projectDir)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if !results[0].Passed {
		t.Errorf("expected Passed=true, got false: %s", results[0].Message)
	}
}

func TestVerify_DetectsMissing(t *testing.T) {
	projectDir := t.TempDir()
	// No settings file created.
	results, err := New().Verify(claude.New(), "", projectDir)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("expected Passed=false when file missing")
	}
}

func TestVerify_DetectsMissingMarker(t *testing.T) {
	projectDir := t.TempDir()
	settingsDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatal(err)
	}

	// File exists but no marker.
	existing := map[string]interface{}{"someKey": "someVal"}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.json"), data, 0644); err != nil {
		t.Fatal(err)
	}

	results, err := New().Verify(claude.New(), "", projectDir)
	if err != nil {
		t.Fatalf("Verify() error: %v", err)
	}
	if results[0].Passed {
		t.Error("expected Passed=false when marker absent")
	}
}

// ─── Strip ──────────────────────────────────────────────────────────────────

func TestStrip_RemovesPermissionsBlock(t *testing.T) {
	projectDir := t.TempDir()
	settingsDir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		t.Fatal(err)
	}
	settingsFile := filepath.Join(settingsDir, "settings.json")

	existing := map[string]interface{}{
		"userKey":     "kept",
		"permissions": map[string]interface{}{"deny": []interface{}{}},
		metaKey:       metaValue,
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(settingsFile, data, 0644); err != nil {
		t.Fatal(err)
	}

	if err := Strip(settingsFile, domain.AgentClaudeCode); err != nil {
		t.Fatalf("Strip() error: %v", err)
	}

	result := readJSON(t, settingsFile)
	if _, ok := result["permissions"]; ok {
		t.Error("expected 'permissions' key to be removed")
	}
	if _, ok := result[metaKey]; ok {
		t.Error("expected meta key to be removed")
	}
	if result["userKey"] != "kept" {
		t.Error("user key 'userKey' was removed")
	}
}

// ─── Cursor/Windsurf graceful handling ─────────────────────────────────────

func TestPlan_CursorReturnsNoAction(t *testing.T) {
	projectDir := t.TempDir()
	actions, err := New().Plan(cursor.New(), "", projectDir)
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}
	for _, a := range actions {
		if a.Action != domain.ActionSkip {
			t.Errorf("expected no non-skip actions for cursor, got %s", a.Action)
		}
	}
}

func TestPlan_WindsurfReturnsNoAction(t *testing.T) {
	projectDir := t.TempDir()
	actions, err := New().Plan(windsurf.New(), "", projectDir)
	if err != nil {
		t.Fatalf("Plan() error: %v", err)
	}
	for _, a := range actions {
		if a.Action != domain.ActionSkip {
			t.Errorf("expected no non-skip actions for windsurf, got %s", a.Action)
		}
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func readJSON(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("readJSON: %v", err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("readJSON unmarshal: %v", err)
	}
	return out
}
