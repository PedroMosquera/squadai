package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/claude"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// ─── Interface compliance ───────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	inst := New(nil)
	if inst.ID() != domain.ComponentMCP {
		t.Errorf("ID() = %q, want %q", inst.ID(), domain.ComponentMCP)
	}
}

// ─── Constructor ────────────────────────────────────────────────────────────

func TestNew_FiltersDisabledServers(t *testing.T) {
	mcpConfig := map[string]domain.MCPServerDef{
		"enabled-server":  {Type: "remote", URL: "https://a.com/mcp", Enabled: true},
		"disabled-server": {Type: "remote", URL: "https://b.com/mcp", Enabled: false},
	}
	inst := New(mcpConfig)
	if len(inst.Servers()) != 1 {
		t.Errorf("expected 1 enabled server, got %d", len(inst.Servers()))
	}
	if _, ok := inst.Servers()["enabled-server"]; !ok {
		t.Error("enabled-server should be present")
	}
}

func TestNew_NilConfig(t *testing.T) {
	inst := New(nil)
	if len(inst.Servers()) != 0 {
		t.Error("nil config should produce no servers")
	}
}

// ─── Plan (OpenCode — MergeIntoSettings) ────────────────────────────────────

func TestPlan_OpenCode_NewFile_ReturnsCreate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller()

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionCreate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionCreate)
	}
	expected := filepath.Join(project, "opencode.json")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
	if actions[0].Component != domain.ComponentMCP {
		t.Errorf("Component = %q, want %q", actions[0].Component, domain.ComponentMCP)
	}
}

func TestPlan_OpenCode_UpToDate_ReturnsSkip(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller()

	// Write existing file with matching MCP config.
	targetPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"mcp": map[string]interface{}{
			"context7": map[string]interface{}{
				"type": "remote",
				"url":  "https://mcp.context7.com/mcp",
			},
		},
	})

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionSkip {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionSkip)
	}
}

func TestPlan_OpenCode_OutdatedMCP_ReturnsUpdate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller()

	// Write file with different MCP config.
	targetPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"mcp": map[string]interface{}{
			"context7": map[string]interface{}{
				"type": "remote",
				"url":  "https://old-url.com/mcp",
			},
		},
	})

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actions[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionUpdate)
	}
}

func TestPlan_OpenCode_MissingMCPKey_ReturnsUpdate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller()

	// Write file without mcp key.
	targetPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"model": "some-model",
	})

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actions[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionUpdate)
	}
}

// ─── Plan (Claude Code — SeparateMCPFiles) ──────────────────────────────────

func TestPlan_Claude_SeparateFiles_ReturnsCreate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller()

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action (one server), got %d", len(actions))
	}
	if actions[0].Action != domain.ActionCreate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionCreate)
	}
	// Should target ~/.claude/mcp/context7.json
	expected := filepath.Join(home, ".claude", "mcp", "context7.json")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestPlan_Claude_SeparateFiles_UpToDate_ReturnsSkip(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller()

	// Write existing file matching expected content.
	mcpDir := filepath.Join(home, ".claude", "mcp")
	if err := os.MkdirAll(mcpDir, 0755); err != nil {
		t.Fatal(err)
	}
	expected := serverToJSON(domain.MCPServerDef{
		Type: "remote",
		URL:  "https://mcp.context7.com/mcp",
	})
	if err := os.WriteFile(filepath.Join(mcpDir, "context7.json"), expected, 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, home, project)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionSkip {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionSkip)
	}
}

func TestPlan_Claude_SeparateFiles_Outdated_ReturnsUpdate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller()

	// Write existing file with wrong content.
	mcpDir := filepath.Join(home, ".claude", "mcp")
	if err := os.MkdirAll(mcpDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mcpDir, "context7.json"), []byte(`{"type":"remote","url":"https://old.com"}`), 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, home, project)
	if actions[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionUpdate)
	}
}

func TestPlan_Claude_MultipleServers_SeparateFiles(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := New(map[string]domain.MCPServerDef{
		"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Enabled: true},
		"sentry":   {Type: "remote", URL: "https://mcp.sentry.dev", Enabled: true},
	})

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions (one per server), got %d", len(actions))
	}

	// Both should be Create actions targeting separate files.
	for _, a := range actions {
		if a.Action != domain.ActionCreate {
			t.Errorf("Action = %q, want %q", a.Action, domain.ActionCreate)
		}
		dir := filepath.Dir(a.TargetPath)
		if filepath.Base(dir) != "mcp" {
			t.Errorf("file should be in mcp/ dir, got %s", dir)
		}
	}
}

// ─── Plan (no servers) ──────────────────────────────────────────────────────

func TestPlan_NoServers_ReturnsNil(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(nil)

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for empty servers, got %d", len(actions))
	}
}

// ─── Apply (OpenCode — MergeIntoSettings) ───────────────────────────────────

func TestApply_OpenCode_CreatesFileWithMCPServers(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller()

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	doc := readTestJSON(t, actions[0].TargetPath)
	mcpMap, ok := doc["mcp"].(map[string]interface{})
	if !ok {
		t.Fatal("mcp key should be a map")
	}
	server, ok := mcpMap["context7"].(map[string]interface{})
	if !ok {
		t.Fatal("context7 server should be present")
	}
	if server["type"] != "remote" {
		t.Errorf("type = %v, want remote", server["type"])
	}
	if server["url"] != "https://mcp.context7.com/mcp" {
		t.Errorf("url = %v, want https://mcp.context7.com/mcp", server["url"])
	}

	// Check managed keys metadata.
	meta, ok := doc[managedMetaKey].(map[string]interface{})
	if !ok {
		t.Fatal("_agent_manager should be present")
	}
	keys, ok := meta["managed_keys"].([]interface{})
	if !ok {
		t.Fatal("managed_keys should be an array")
	}
	foundMCP := false
	for _, k := range keys {
		if k == "mcp" {
			foundMCP = true
		}
	}
	if !foundMCP {
		t.Error("managed_keys should include 'mcp'")
	}
}

func TestApply_PreservesExistingKeys(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller()

	// Write file with existing settings.
	targetPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"model":    "anthropic/claude-sonnet-4-5",
		"provider": "anthropic",
		managedMetaKey: map[string]interface{}{
			"managed_keys": []string{"model", "provider"},
		},
	})

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, targetPath)
	if doc["model"] != "anthropic/claude-sonnet-4-5" {
		t.Error("model should be preserved")
	}
	if doc["provider"] != "anthropic" {
		t.Error("provider should be preserved")
	}
	if _, ok := doc["mcp"].(map[string]interface{}); !ok {
		t.Error("mcp key should be written")
	}

	// Check managed_keys includes both existing and mcp.
	meta := doc[managedMetaKey].(map[string]interface{})
	keys := meta["managed_keys"].([]interface{})
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k.(string)] = true
	}
	if !keySet["model"] {
		t.Error("managed_keys should include 'model'")
	}
	if !keySet["mcp"] {
		t.Error("managed_keys should include 'mcp'")
	}
}

func TestApply_LocalServerWithCommand(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(map[string]domain.MCPServerDef{
		"local-db": {
			Type:    "local",
			Command: []string{"npx", "-y", "@modelcontextprotocol/server-postgres"},
			Enabled: true,
			Environment: map[string]string{
				"DATABASE_URL": "postgres://localhost/mydb",
			},
		},
	})

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, actions[0].TargetPath)
	mcpMap := doc["mcp"].(map[string]interface{})
	server := mcpMap["local-db"].(map[string]interface{})
	if server["type"] != "local" {
		t.Error("type should be local")
	}
	cmd, ok := server["command"].([]interface{})
	if !ok || len(cmd) != 3 {
		t.Error("command should be an array of 3 elements")
	}
	env, ok := server["environment"].(map[string]interface{})
	if !ok {
		t.Fatal("environment should be present")
	}
	if env["DATABASE_URL"] != "postgres://localhost/mydb" {
		t.Error("environment variable should match")
	}
}

func TestApply_SkipDoesNothing(t *testing.T) {
	inst := New(nil)
	err := inst.Apply(domain.PlannedAction{Action: domain.ActionSkip})
	if err != nil {
		t.Fatalf("Skip should succeed, got: %v", err)
	}
}

func TestApply_Idempotent(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller()

	// First apply.
	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(actions[0].TargetPath)

	// Second plan — should be Skip.
	actions2, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions2[0].Action != domain.ActionSkip {
		t.Fatalf("second plan should be Skip, got %q", actions2[0].Action)
	}

	second, _ := os.ReadFile(actions[0].TargetPath)
	if string(first) != string(second) {
		t.Error("content should not change on second plan")
	}
}

func TestApply_UpdatesOutdatedMCP(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()

	// Write old MCP config.
	targetPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"mcp": map[string]interface{}{
			"old-server": map[string]interface{}{
				"type": "remote",
				"url":  "https://old.com/mcp",
			},
		},
	})

	// Install new MCP config.
	inst := newTestInstaller()
	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions[0].Action != domain.ActionUpdate {
		t.Fatalf("expected Update, got %q", actions[0].Action)
	}
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, targetPath)
	mcpMap := doc["mcp"].(map[string]interface{})
	if _, ok := mcpMap["old-server"]; ok {
		t.Error("old-server should be replaced")
	}
	if _, ok := mcpMap["context7"]; !ok {
		t.Error("context7 should be present")
	}
}

// ─── Apply (Claude Code — SeparateMCPFiles) ─────────────────────────────────

func TestApply_Claude_SeparateFile_CreatesFile(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller()

	actions, _ := inst.Plan(adapter, home, project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, err := os.ReadFile(actions[0].TargetPath)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if doc["type"] != "remote" {
		t.Errorf("type = %v, want remote", doc["type"])
	}
	if doc["url"] != "https://mcp.context7.com/mcp" {
		t.Errorf("url = %v, want https://mcp.context7.com/mcp", doc["url"])
	}
}

func TestApply_Claude_SeparateFile_Idempotent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller()

	// First apply.
	actions, _ := inst.Plan(adapter, home, project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(actions[0].TargetPath)

	// Second plan — should be Skip.
	actions2, _ := inst.Plan(adapter, home, project)
	if actions2[0].Action != domain.ActionSkip {
		t.Fatalf("second plan should be Skip, got %q", actions2[0].Action)
	}

	second, _ := os.ReadFile(actions[0].TargetPath)
	if string(first) != string(second) {
		t.Error("content should not change")
	}
}

// ─── Verify (OpenCode — MergeIntoSettings) ──────────────────────────────────

func TestVerify_OpenCode_AllPass_AfterApply(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller()

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	results, err := inst.Verify(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
	if len(results) != 2 {
		t.Errorf("expected 2 verify checks, got %d", len(results))
	}
}

func TestVerify_FailsWhenFileMissing(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller()

	results, err := inst.Verify(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("expected mcp-file-exists check to fail")
	}
}

func TestVerify_FailsWhenMCPOutdated(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller()

	// Write file with wrong MCP config.
	targetPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"mcp": map[string]interface{}{
			"context7": map[string]interface{}{
				"type": "remote",
				"url":  "https://wrong-url.com/mcp",
			},
		},
	})

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	foundCheck := false
	for _, r := range results {
		if r.Check == "mcp-servers-current" {
			foundCheck = true
			if r.Passed {
				t.Error("mcp-servers-current should fail when MCP config is outdated")
			}
		}
	}
	if !foundCheck {
		t.Error("expected mcp-servers-current check in results")
	}
}

func TestVerify_NoServers_ReturnsNil(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(nil)

	results, err := inst.Verify(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty servers, got %d", len(results))
	}
}

// ─── Verify (Claude Code — SeparateMCPFiles) ────────────────────────────────

func TestVerify_Claude_SeparateFiles_AllPass(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller()

	actions, _ := inst.Plan(adapter, home, project)
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatal(err)
		}
	}

	results, err := inst.Verify(adapter, home, project)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
	if len(results) != 2 {
		t.Errorf("expected 2 verify checks, got %d", len(results))
	}
}

func TestVerify_Claude_SeparateFiles_FailsWhenMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller()

	results, _ := inst.Verify(adapter, home, project)
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("should fail when MCP file missing")
	}
}

// ─── serverToMap ────────────────────────────────────────────────────────────

func TestServerToMap_RemoteServer(t *testing.T) {
	def := domain.MCPServerDef{
		Type: "remote",
		URL:  "https://mcp.example.com",
	}
	m := serverToMap(def)
	if m["type"] != "remote" {
		t.Error("type should be remote")
	}
	if m["url"] != "https://mcp.example.com" {
		t.Error("url should be set")
	}
	if _, ok := m["command"]; ok {
		t.Error("command should not be set for remote")
	}
}

func TestServerToMap_LocalServer(t *testing.T) {
	def := domain.MCPServerDef{
		Type:    "local",
		Command: []string{"npx", "server"},
		Environment: map[string]string{
			"KEY": "value",
		},
	}
	m := serverToMap(def)
	if m["type"] != "local" {
		t.Error("type should be local")
	}
	if _, ok := m["url"]; ok {
		t.Error("url should not be set for local")
	}
	cmd, ok := m["command"].([]string)
	if !ok || len(cmd) != 2 {
		t.Error("command should be present")
	}
	env, ok := m["environment"].(map[string]string)
	if !ok || env["KEY"] != "value" {
		t.Error("environment should be present")
	}
}

// ─── updateManagedKeys ──────────────────────────────────────────────────────

func TestUpdateManagedKeys_AddsNew(t *testing.T) {
	doc := map[string]interface{}{}
	updateManagedKeys(doc, "mcp")

	meta := doc[managedMetaKey].(map[string]interface{})
	keys := meta["managed_keys"].([]string)
	if len(keys) != 1 || keys[0] != "mcp" {
		t.Errorf("expected [mcp], got %v", keys)
	}
}

func TestUpdateManagedKeys_PreservesExisting(t *testing.T) {
	doc := map[string]interface{}{
		managedMetaKey: map[string]interface{}{
			"managed_keys": []interface{}{"model", "provider"},
		},
	}
	updateManagedKeys(doc, "mcp")

	meta := doc[managedMetaKey].(map[string]interface{})
	keys := meta["managed_keys"].([]string)
	if len(keys) != 3 {
		t.Errorf("expected 3 keys, got %d", len(keys))
	}
}

func TestUpdateManagedKeys_NoDuplicate(t *testing.T) {
	doc := map[string]interface{}{
		managedMetaKey: map[string]interface{}{
			"managed_keys": []interface{}{"mcp", "model"},
		},
	}
	updateManagedKeys(doc, "mcp")

	meta := doc[managedMetaKey].(map[string]interface{})
	keys := meta["managed_keys"].([]string)
	if len(keys) != 2 {
		t.Errorf("expected 2 keys (no duplicate), got %d", len(keys))
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func newTestInstaller() *Installer {
	return New(map[string]domain.MCPServerDef{
		"context7": {
			Type:    "remote",
			URL:     "https://mcp.context7.com/mcp",
			Enabled: true,
		},
	})
}

func writeTestJSON(t *testing.T, path string, data map[string]interface{}) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
}

func readTestJSON(t *testing.T, path string) map[string]interface{} {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
