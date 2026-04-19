package mcp

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
	"github.com/PedroMosquera/squadai/internal/managed"
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

// ─── Plan (Claude Code — MCPConfigFile) ─────────────────────────────────────

func TestPlan_Claude_MCPConfigFile_ReturnsCreate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller()

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionCreate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionCreate)
	}
	// Should target <project>/.mcp.json
	expected := filepath.Join(project, ".mcp.json")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestPlan_Claude_MCPConfigFile_UpToDate_ReturnsSkip(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller()

	// Write existing .mcp.json with matching content.
	targetPath := filepath.Join(project, ".mcp.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"context7": map[string]interface{}{
				"url": "https://mcp.context7.com/mcp",
			},
		},
	})

	actions, _ := inst.Plan(adapter, home, project)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionSkip {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionSkip)
	}
}

func TestPlan_Claude_MCPConfigFile_Outdated_ReturnsUpdate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller()

	// Write existing .mcp.json with wrong content.
	targetPath := filepath.Join(project, ".mcp.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"context7": map[string]interface{}{
				"url": "https://wrong.com",
			},
		},
	})

	actions, _ := inst.Plan(adapter, home, project)
	if actions[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionUpdate)
	}
}

func TestPlan_Claude_MCPConfigFile_MultipleServers(t *testing.T) {
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
	if len(actions) != 1 {
		t.Fatalf("expected 1 action (single config file), got %d", len(actions))
	}
	if actions[0].Action != domain.ActionCreate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionCreate)
	}
	// Target should be .mcp.json at project root.
	expected := filepath.Join(project, ".mcp.json")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
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

	// Check that _agent_manager is NOT written into the config doc.
	if _, hasKey := doc["_agent_manager"]; hasKey {
		t.Error("_agent_manager must not appear in the config file — tracking moved to sidecar")
	}

	// Check managed keys are written to the sidecar.
	sidecarKeys, err := managed.ReadManagedKeys(project, "opencode.json")
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	foundMCP := false
	for _, k := range sidecarKeys {
		if k == "mcp" {
			foundMCP = true
		}
	}
	if !foundMCP {
		t.Error("sidecar managed_keys should include 'mcp'")
	}
}

func TestApply_PreservesExistingKeys(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller()

	// Write file with existing settings (no _agent_manager — tracking is now in sidecar).
	targetPath := filepath.Join(project, "opencode.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"model":    "anthropic/claude-sonnet-4-5",
		"provider": "anthropic",
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

	// _agent_manager must NOT be in the config file.
	if _, hasKey := doc["_agent_manager"]; hasKey {
		t.Error("_agent_manager must not appear in the config file — tracking moved to sidecar")
	}

	// Check sidecar has 'mcp' in managed_keys.
	sidecarKeys, err := managed.ReadManagedKeys(project, "opencode.json")
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	foundMCP := false
	for _, k := range sidecarKeys {
		if k == "mcp" {
			foundMCP = true
		}
	}
	if !foundMCP {
		t.Error("sidecar managed_keys should include 'mcp'")
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

// ─── Apply (Claude Code — MCPConfigFile) ─────────────────────────────────────

func TestApply_Claude_MCPConfigFile_CreatesFile(t *testing.T) {
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

	targetPath := filepath.Join(project, ".mcp.json")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	servers, ok := doc["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected mcpServers key, got %T", doc["mcpServers"])
	}
	ctx7, ok := servers["context7"].(map[string]interface{})
	if !ok {
		t.Fatal("expected context7 server")
	}
	if ctx7["url"] != "https://mcp.context7.com/mcp" {
		t.Errorf("url = %v, want https://mcp.context7.com/mcp", ctx7["url"])
	}
	// Claude Code format: no "type" field, command is string + args.
	if _, hasType := ctx7["type"]; hasType {
		t.Error("Claude Code format must NOT include 'type' field")
	}
}

func TestApply_Claude_MCPConfigFile_Idempotent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller()

	// First apply.
	actions, _ := inst.Plan(adapter, home, project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}
	targetPath := filepath.Join(project, ".mcp.json")
	first, _ := os.ReadFile(targetPath)

	// Second plan — should be Skip.
	actions2, _ := inst.Plan(adapter, home, project)
	if actions2[0].Action != domain.ActionSkip {
		t.Fatalf("second plan should be Skip, got %q", actions2[0].Action)
	}

	second, _ := os.ReadFile(targetPath)
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

// ─── Verify (Claude Code — MCPConfigFile) ────────────────────────────────────

func TestVerify_Claude_MCPConfigFile_AllPass(t *testing.T) {
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

func TestVerify_Claude_MCPConfigFile_FailsWhenMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := claude.New()
	inst := newTestInstaller()

	results, _ := inst.Verify(adapter, home, project)
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("should fail when .mcp.json is missing")
	}
}

// ─── serverToMap ────────────────────────────────────────────────────────────

func TestServerToMap_RemoteServer(t *testing.T) {
	def := domain.MCPServerDef{
		Type: "remote",
		URL:  "https://mcp.example.com",
	}
	m := serverToMap(def, domain.AgentOpenCode)
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
	m := serverToMap(def, domain.AgentOpenCode)
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

// ─── Fix 5: Non-OpenCode agents use split command/args format ───────────────

func TestServerToMap_ClaudeCode_SplitCommandArgs(t *testing.T) {
	def := domain.MCPServerDef{
		Type:    "local",
		Command: []string{"npx", "-y", "@upstash/context7-mcp@latest"},
	}
	m := serverToMap(def, domain.AgentClaudeCode)

	// Claude Code: command is a string (first element), args is the rest.
	if m["command"] != "npx" {
		t.Errorf("command = %v, want \"npx\" (string, not array)", m["command"])
	}
	args, ok := m["args"].([]string)
	if !ok || len(args) != 2 {
		t.Fatalf("args should be []string with 2 elements, got %T %v", m["args"], m["args"])
	}
	if args[0] != "-y" || args[1] != "@upstash/context7-mcp@latest" {
		t.Errorf("args = %v, want [-y @upstash/context7-mcp@latest]", args)
	}
	// No "type" field for Claude Code.
	if _, hasType := m["type"]; hasType {
		t.Error("Claude Code format must NOT include 'type' field")
	}
}

func TestServerToMap_ClaudeCode_RemoteServer(t *testing.T) {
	def := domain.MCPServerDef{
		Type: "remote",
		URL:  "https://mcp.context7.com/mcp",
	}
	m := serverToMap(def, domain.AgentClaudeCode)

	// No type, no command, just url.
	if _, hasType := m["type"]; hasType {
		t.Error("Claude Code remote must NOT include 'type' field")
	}
	if m["url"] != "https://mcp.context7.com/mcp" {
		t.Errorf("url = %v, want https://mcp.context7.com/mcp", m["url"])
	}
}

func TestServerToMap_SingleElementCommand(t *testing.T) {
	def := domain.MCPServerDef{
		Type:    "local",
		Command: []string{"my-server"},
	}
	m := serverToMap(def, domain.AgentCursor)

	if m["command"] != "my-server" {
		t.Errorf("command = %v, want \"my-server\"", m["command"])
	}
	// No args when command has only one element.
	if _, hasArgs := m["args"]; hasArgs {
		t.Error("args should not be present for single-element command")
	}
}

// ─── Sidecar tracking (replaces TestUpdateManagedKeys_* after _agent_manager removal) ─

func TestApply_OpenCode_WritesSidecar(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller()

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	keys, err := managed.ReadManagedKeys(project, "opencode.json")
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	if len(keys) != 1 || keys[0] != "mcp" {
		t.Errorf("expected [mcp] in sidecar, got %v", keys)
	}
}

func TestApply_OpenCode_SidecarPreservesOnRepeat(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := newTestInstaller()

	// First apply.
	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	// Second apply (after changing servers — simulate an update).
	inst2 := New(map[string]domain.MCPServerDef{
		"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Enabled: true},
		"sentry":   {Type: "remote", URL: "https://mcp.sentry.dev", Enabled: true},
	})
	actions2, _ := inst2.Plan(adapter, t.TempDir(), project)
	if len(actions2) > 0 && actions2[0].Action != domain.ActionSkip {
		if err := inst2.Apply(actions2[0]); err != nil {
			t.Fatal(err)
		}
	}

	keys, err := managed.ReadManagedKeys(project, "opencode.json")
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	if len(keys) == 0 {
		t.Error("sidecar should have managed keys after apply")
	}
}

func TestApply_NoProjectDir_SidecarNotWritten(t *testing.T) {
	// When Apply is called without a prior Plan (no projectDir set),
	// the sidecar write is skipped gracefully — no panic, no error.
	project := t.TempDir()
	inst := newTestInstaller()

	targetPath := filepath.Join(project, "opencode.json")
	action := domain.PlannedAction{
		ID:         "opencode-mcp",
		Agent:      "opencode",
		Component:  domain.ComponentMCP,
		Action:     domain.ActionCreate,
		TargetPath: targetPath,
	}

	if err := inst.Apply(action); err != nil {
		t.Fatalf("Apply without projectDir should not fail: %v", err)
	}
}

// ─── Plan (VS Code — MCPConfigFile) ─────────────────────────────────────────

func TestPlan_VSCode_MCPConfigFile_ReturnsCreate(t *testing.T) {
	project := t.TempDir()
	adapter := vscode.New()
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
	// VS Code MCP goes to .vscode/mcp.json, NOT .vscode/settings.json.
	expected := filepath.Join(project, ".vscode", "mcp.json")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
	if actions[0].Component != domain.ComponentMCP {
		t.Errorf("Component = %q, want %q", actions[0].Component, domain.ComponentMCP)
	}
}

func TestPlan_Cursor_MCPConfigFile_ReturnsCreate(t *testing.T) {
	project := t.TempDir()
	adapter := cursor.New()
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
	expected := filepath.Join(project, ".cursor", "mcp.json")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestPlan_Windsurf_MCPConfigFile_ReturnsCreate(t *testing.T) {
	project := t.TempDir()
	adapter := windsurf.New()
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
	expected := filepath.Join(project, ".windsurf", "mcp_config.json")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestPlan_VSCode_MCPConfigFile_UpToDate_ReturnsSkip(t *testing.T) {
	project := t.TempDir()
	adapter := vscode.New()
	inst := newTestInstaller()

	targetPath := filepath.Join(project, ".vscode", "mcp.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"servers": map[string]interface{}{
			"context7": map[string]interface{}{
				"url": "https://mcp.context7.com/mcp",
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

func TestPlan_VSCode_MCPConfigFile_Outdated_ReturnsUpdate(t *testing.T) {
	project := t.TempDir()
	adapter := vscode.New()
	inst := newTestInstaller()

	targetPath := filepath.Join(project, ".vscode", "mcp.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"servers": map[string]interface{}{
			"context7": map[string]interface{}{
				"url": "https://old-url.com/mcp",
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

// ─── Apply (MCPConfigFile strategy) ─────────────────────────────────────────

func TestApply_VSCode_MCPConfigFile_CreatesFileWithMcpServers(t *testing.T) {
	project := t.TempDir()
	adapter := vscode.New()
	inst := newTestInstaller()

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	doc := readTestJSON(t, actions[0].TargetPath)
	// VS Code Copilot MUST use "servers" key, NOT "mcpServers".
	serversMap, ok := doc["servers"].(map[string]interface{})
	if !ok {
		t.Fatal("servers key should be a map")
	}
	if _, hasMCPServers := doc["mcpServers"]; hasMCPServers {
		t.Error("should NOT have 'mcpServers' key — VS Code uses 'servers'")
	}
	if _, hasMCP := doc["mcp"]; hasMCP {
		t.Error("should NOT have 'mcp' key — MCPConfigFile uses 'servers' for VS Code")
	}
	server, ok := serversMap["context7"].(map[string]interface{})
	if !ok {
		t.Fatal("context7 server should be present")
	}
	if server["url"] != "https://mcp.context7.com/mcp" {
		t.Errorf("url = %v, want https://mcp.context7.com/mcp", server["url"])
	}
	// VS Code format: no "type" field.
	if _, hasType := server["type"]; hasType {
		t.Error("VS Code format must NOT include 'type' field")
	}

	// Check that _agent_manager is NOT written into the config doc.
	if _, hasKey := doc["_agent_manager"]; hasKey {
		t.Error("_agent_manager must not appear in the config file — tracking moved to sidecar")
	}

	// Check managed keys are written to the sidecar (relative path from project root).
	sidecarKeys, err := managed.ReadManagedKeys(project, filepath.Join(".vscode", "mcp.json"))
	if err != nil {
		t.Fatalf("read sidecar: %v", err)
	}
	foundKey := false
	for _, k := range sidecarKeys {
		if k == "servers" {
			foundKey = true
		}
	}
	if !foundKey {
		t.Error("sidecar managed_keys should include 'servers'")
	}
}

func TestApply_Cursor_MCPConfigFile_PreservesExistingKeys(t *testing.T) {
	project := t.TempDir()
	adapter := cursor.New()
	inst := newTestInstaller()

	targetPath := filepath.Join(project, ".cursor", "mcp.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"existingKey": "existingValue",
	})

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, targetPath)
	if doc["existingKey"] != "existingValue" {
		t.Error("existingKey should be preserved")
	}
	if _, ok := doc["mcpServers"].(map[string]interface{}); !ok {
		t.Error("mcpServers key should be written")
	}
}

func TestApply_Windsurf_MCPConfigFile_MultipleServers(t *testing.T) {
	project := t.TempDir()
	adapter := windsurf.New()
	inst := New(map[string]domain.MCPServerDef{
		"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Enabled: true},
		"sentry":   {Type: "remote", URL: "https://mcp.sentry.dev", Enabled: true},
	})

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) != 1 {
		t.Fatalf("MCPConfigFile should produce 1 action for all servers, got %d", len(actions))
	}

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, actions[0].TargetPath)
	serversMap := doc["mcpServers"].(map[string]interface{})
	if len(serversMap) != 2 {
		t.Errorf("expected 2 servers, got %d", len(serversMap))
	}
	if _, ok := serversMap["context7"]; !ok {
		t.Error("context7 should be present")
	}
	if _, ok := serversMap["sentry"]; !ok {
		t.Error("sentry should be present")
	}
}

func TestApply_VSCode_MCPConfigFile_Idempotent(t *testing.T) {
	project := t.TempDir()
	adapter := vscode.New()
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

// ─── Verify (MCPConfigFile strategy) ────────────────────────────────────────

func TestVerify_VSCode_MCPConfigFile_AllPass(t *testing.T) {
	project := t.TempDir()
	adapter := vscode.New()
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

func TestVerify_Cursor_MCPConfigFile_FailsWhenMissing(t *testing.T) {
	project := t.TempDir()
	adapter := cursor.New()
	inst := newTestInstaller()

	results, err := inst.Verify(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("expected mcp-configfile-exists check to fail")
	}
}

func TestVerify_Windsurf_MCPConfigFile_FailsWhenOutdated(t *testing.T) {
	project := t.TempDir()
	adapter := windsurf.New()
	inst := newTestInstaller()

	// Write file with wrong mcpServers config — note Windsurf uses "serverUrl" for remote.
	targetPath := filepath.Join(project, ".windsurf", "mcp_config.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"context7": map[string]interface{}{
				"serverUrl": "https://wrong-url.com/mcp",
			},
		},
	})

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	foundCheck := false
	for _, r := range results {
		if r.Check == "mcp-configfile-servers-current" {
			foundCheck = true
			if r.Passed {
				t.Error("mcp-configfile-servers-current should fail when config is outdated")
			}
		}
	}
	if !foundCheck {
		t.Error("expected mcp-configfile-servers-current check in results")
	}
}

// ─── Fix 1: VS Code uses "servers" root key ─────────────────────────────────

func TestApply_VSCode_UsesServersRootKey(t *testing.T) {
	project := t.TempDir()
	adapter := vscode.New()
	inst := newTestInstaller()

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, actions[0].TargetPath)
	if _, ok := doc["servers"]; !ok {
		t.Error("VS Code MUST use 'servers' root key")
	}
	if _, ok := doc["mcpServers"]; ok {
		t.Error("VS Code MUST NOT use 'mcpServers' root key")
	}
}

func TestApply_Cursor_UsesMcpServersRootKey(t *testing.T) {
	project := t.TempDir()
	adapter := cursor.New()
	inst := newTestInstaller()

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, actions[0].TargetPath)
	if _, ok := doc["mcpServers"]; !ok {
		t.Error("Cursor MUST use 'mcpServers' root key")
	}
	if _, ok := doc["servers"]; ok {
		t.Error("Cursor MUST NOT use 'servers' root key")
	}
}

// ─── Fix 2: Windsurf uses "serverUrl" for remote servers ────────────────────

func TestApply_Windsurf_UsesServerUrlField(t *testing.T) {
	project := t.TempDir()
	adapter := windsurf.New()
	inst := newTestInstaller()

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, actions[0].TargetPath)
	serversMap := doc["mcpServers"].(map[string]interface{})
	server := serversMap["context7"].(map[string]interface{})
	if _, ok := server["serverUrl"]; !ok {
		t.Error("Windsurf remote servers MUST use 'serverUrl' field")
	}
	if _, ok := server["url"]; ok {
		t.Error("Windsurf remote servers MUST NOT use 'url' field")
	}
	if server["serverUrl"] != "https://mcp.context7.com/mcp" {
		t.Errorf("serverUrl = %v, want https://mcp.context7.com/mcp", server["serverUrl"])
	}
}

func TestApply_Cursor_UsesUrlField(t *testing.T) {
	project := t.TempDir()
	adapter := cursor.New()
	inst := newTestInstaller()

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, actions[0].TargetPath)
	serversMap := doc["mcpServers"].(map[string]interface{})
	server := serversMap["context7"].(map[string]interface{})
	if _, ok := server["url"]; !ok {
		t.Error("Cursor remote servers MUST use 'url' field")
	}
	if _, ok := server["serverUrl"]; ok {
		t.Error("Cursor remote servers MUST NOT use 'serverUrl' field")
	}
}

// ─── Fix 3: OpenCode MCP uses "mcp" key with correct format ────────────────

func TestApply_OpenCode_UsesMcpRootKey_CorrectFormat(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(map[string]domain.MCPServerDef{
		"local-server": {
			Type:    "local",
			Command: []string{"npx", "-y", "@modelcontextprotocol/server-postgres"},
			Environment: map[string]string{
				"DATABASE_URL": "postgres://localhost/mydb",
			},
			Enabled: true,
		},
		"remote-server": {
			Type:    "remote",
			URL:     "https://mcp.example.com",
			Enabled: true,
			Headers: map[string]string{"Authorization": "Bearer token"},
		},
	})

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, actions[0].TargetPath)
	// OpenCode MUST use "mcp" root key.
	mcpMap, ok := doc["mcp"].(map[string]interface{})
	if !ok {
		t.Fatal("OpenCode MUST use 'mcp' root key")
	}
	if _, ok := doc["mcpServers"]; ok {
		t.Error("OpenCode MUST NOT use 'mcpServers' root key")
	}

	// Local server format: "command" as array, "environment" (not "env").
	local := mcpMap["local-server"].(map[string]interface{})
	if local["type"] != "local" {
		t.Error("local server type should be 'local'")
	}
	cmd, ok := local["command"].([]interface{})
	if !ok || len(cmd) != 3 {
		t.Error("command should be an array of 3 elements")
	}
	env, ok := local["environment"].(map[string]interface{})
	if !ok {
		t.Fatal("should use 'environment' key (not 'env')")
	}
	if env["DATABASE_URL"] != "postgres://localhost/mydb" {
		t.Error("environment variable should match")
	}

	// Remote server format: "url" (not "serverUrl").
	remote := mcpMap["remote-server"].(map[string]interface{})
	if remote["type"] != "remote" {
		t.Error("remote server type should be 'remote'")
	}
	if remote["url"] != "https://mcp.example.com" {
		t.Error("remote server should use 'url' field")
	}
	if _, ok := remote["serverUrl"]; ok {
		t.Error("OpenCode MUST NOT use 'serverUrl' field")
	}
	headers, ok := remote["headers"].(map[string]interface{})
	if !ok || headers["Authorization"] != "Bearer token" {
		t.Error("headers should be present")
	}
}

// ─── Fix 4: VS Code preserves "inputs" array ───────────────────────────────

func TestApply_VSCode_PreservesInputsArray(t *testing.T) {
	project := t.TempDir()
	adapter := vscode.New()
	inst := newTestInstaller()

	// Pre-write mcp.json with an "inputs" array (VS Code credential prompting).
	targetPath := filepath.Join(project, ".vscode", "mcp.json")
	writeTestJSON(t, targetPath, map[string]interface{}{
		"inputs": []interface{}{
			map[string]interface{}{
				"type":        "promptString",
				"id":          "api-key",
				"description": "API Key for MCP server",
				"password":    true,
			},
		},
	})

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	doc := readTestJSON(t, actions[0].TargetPath)

	// "servers" key must be present.
	if _, ok := doc["servers"]; !ok {
		t.Error("servers key should be written")
	}

	// "inputs" array must be preserved.
	inputs, ok := doc["inputs"].([]interface{})
	if !ok {
		t.Fatal("inputs array must be preserved")
	}
	if len(inputs) != 1 {
		t.Errorf("expected 1 input entry, got %d", len(inputs))
	}
	entry, ok := inputs[0].(map[string]interface{})
	if !ok {
		t.Fatal("input entry should be a map")
	}
	if entry["id"] != "api-key" {
		t.Error("input entry id should be preserved")
	}
}

// ─── Round-trip: all 5 agents produce correct format ────────────────────────

func TestRoundTrip_AllAgents_CorrectFormat(t *testing.T) {
	servers := map[string]domain.MCPServerDef{
		"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Enabled: true},
		"local-db": {
			Type:        "local",
			Command:     []string{"npx", "-y", "server"},
			Environment: map[string]string{"KEY": "val"},
			Enabled:     true,
		},
	}

	tests := []struct {
		name          string
		adapter       domain.Adapter
		expectRootKey string // expected root key in written file (empty = "mcp" strategy)
		expectURLKey  string // "url" or "serverUrl"
		strategy      string // "merge", "separate", "configfile"
	}{
		{"OpenCode", opencode.New(), "mcp", "url", "merge"},
		{"Claude", claude.New(), "mcpServers", "url", "configfile"},
		{"VSCode", vscode.New(), "servers", "url", "configfile"},
		{"Cursor", cursor.New(), "mcpServers", "url", "configfile"},
		{"Windsurf", windsurf.New(), "mcpServers", "serverUrl", "configfile"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := t.TempDir()
			project := t.TempDir()
			inst := New(servers)

			actions, err := inst.Plan(tt.adapter, home, project)
			if err != nil {
				t.Fatalf("Plan: %v", err)
			}
			if len(actions) == 0 {
				t.Fatal("expected actions")
			}

			for _, a := range actions {
				if a.Action == domain.ActionSkip {
					continue
				}
				if err := inst.Apply(a); err != nil {
					t.Fatalf("Apply: %v", err)
				}
			}

			// For separate file strategy (Claude), check individual files.
			if tt.strategy == "separate" {
				for name, def := range servers {
					path := filepath.Join(home, ".claude", "mcp", name+".json")
					data, err := os.ReadFile(path)
					if err != nil {
						t.Fatalf("read %s: %v", name, err)
					}
					var doc map[string]interface{}
					if err := json.Unmarshal(data, &doc); err != nil {
						t.Fatalf("parse %s: %v", name, err)
					}
					if def.URL != "" {
						if _, ok := doc[tt.expectURLKey]; !ok {
							t.Errorf("%s: expected URL key %q", name, tt.expectURLKey)
						}
					}
				}
				return
			}

			// For merge/configfile strategies, check the main file.
			var targetPath string
			for _, a := range actions {
				targetPath = a.TargetPath
				break
			}
			doc := readTestJSON(t, targetPath)

			rootMap, ok := doc[tt.expectRootKey].(map[string]interface{})
			if !ok {
				t.Fatalf("expected root key %q to be a map, got %T", tt.expectRootKey, doc[tt.expectRootKey])
			}

			// Check remote server uses correct URL key.
			ctx7, ok := rootMap["context7"].(map[string]interface{})
			if !ok {
				t.Fatal("context7 should be present")
			}
			if _, ok := ctx7[tt.expectURLKey]; !ok {
				t.Errorf("remote server should use %q field", tt.expectURLKey)
			}
			// Ensure the WRONG key is not present.
			wrongKey := "serverUrl"
			if tt.expectURLKey == "serverUrl" {
				wrongKey = "url"
			}
			if _, ok := ctx7[wrongKey]; ok {
				t.Errorf("remote server should NOT use %q field", wrongKey)
			}
		})
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

// ─── VS Code inputs preservation ────────────────────────────────────────────

func TestApplyMCPConfigFile_PreservesVSCodeInputs(t *testing.T) {
	dir := t.TempDir()
	vscodeDir := filepath.Join(dir, ".vscode")
	if err := os.MkdirAll(vscodeDir, 0755); err != nil {
		t.Fatal(err)
	}
	mcpPath := filepath.Join(vscodeDir, "mcp.json")

	// Write an existing mcp.json that has a user-defined "inputs" array.
	existing := map[string]interface{}{
		"inputs": []interface{}{
			map[string]interface{}{"id": "myToken", "type": "promptString"},
		},
		"servers": map[string]interface{}{
			"old-server": map[string]interface{}{"url": "https://old.example.com"},
		},
	}
	writeTestJSON(t, mcpPath, existing)

	_ = vscode.New() // ensure adapter package is imported; action.Agent drives the logic
	servers := map[string]domain.MCPServerDef{
		"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Enabled: true},
	}
	inst := New(servers)
	inst.projectDir = dir
	// Seed the agent config cache (normally populated during Plan).
	inst.agentConfigs[domain.AgentVSCodeCopilot] = agentMCPConfig{
		rootKey: "servers",
		urlKey:  "url",
	}

	action := domain.PlannedAction{
		ID:          "vscode-copilot-mcp",
		Agent:       domain.AgentVSCodeCopilot,
		Component:   domain.ComponentMCP,
		Action:      domain.ActionUpdate,
		TargetPath:  mcpPath,
		Description: "mcp:configfile:update MCP server configuration",
	}

	if err := inst.Apply(action); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	result := readTestJSON(t, mcpPath)

	// "inputs" array must still be present and unchanged.
	inputsRaw, ok := result["inputs"]
	if !ok {
		t.Fatal("Apply() removed the 'inputs' key from mcp.json")
	}
	inputs, ok := inputsRaw.([]interface{})
	if !ok || len(inputs) != 1 {
		t.Fatalf("expected 1 input entry, got %v", inputsRaw)
	}
	inputEntry, ok := inputs[0].(map[string]interface{})
	if !ok || inputEntry["id"] != "myToken" {
		t.Errorf("inputs[0] = %v, want id=myToken", inputs[0])
	}

	// "servers" key must have the new server.
	serversRaw, ok := result["servers"]
	if !ok {
		t.Fatal("Apply() removed the 'servers' key")
	}
	serversMap, ok := serversRaw.(map[string]interface{})
	if !ok {
		t.Fatalf("servers is not a map: %T", serversRaw)
	}
	if _, ok := serversMap["context7"]; !ok {
		t.Error("expected 'context7' server to be written")
	}
	// Old server should be replaced (we overwrite the servers key entirely).
	if _, ok := serversMap["old-server"]; ok {
		t.Error("old-server should have been replaced")
	}
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
