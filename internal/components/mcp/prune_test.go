package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/managed"
)

// installServers applies a full MCP install for opencode and returns the
// config path.
func installServers(t *testing.T, project string, servers map[string]domain.MCPServerDef) string {
	t.Helper()
	adapter := opencode.New()
	inst := New(servers)
	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("apply: %v", err)
		}
	}
	return adapter.ProjectConfigFile(project)
}

func readServers(t *testing.T, path string) map[string]any {
	t.Helper()
	doc, err := fileutil.ReadJSONFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if doc == nil {
		return nil
	}
	m, _ := doc["mcp"].(map[string]any)
	return m
}

// TestProfileSwitch_PrunesFilteredServers verifies the profile-switch
// checkpoint: servers previously managed by SquadAI that are absent from the
// filtered config are removed on the next apply.
func TestProfileSwitch_PrunesFilteredServers(t *testing.T) {
	project := t.TempDir()
	full := map[string]domain.MCPServerDef{
		"context7": {Type: "local", Command: []string{"npx", "c7"}, Enabled: true},
		"github":   {Type: "local", Command: []string{"npx", "gh"}, Enabled: true},
	}
	configPath := installServers(t, project, full)

	if got := readServers(t, configPath); len(got) != 2 {
		t.Fatalf("precondition: expected 2 installed servers, got %v", got)
	}

	// Profile filters down to context7 only (non-empty strict filter).
	filtered := map[string]domain.MCPServerDef{
		"context7": full["context7"],
	}
	adapter := opencode.New()
	inst := New(filtered, Options{PruneWhenEmpty: true})
	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(actions) != 1 || actions[0].Action != domain.ActionUpdate {
		t.Fatalf("expected update action, got %+v", actions)
	}
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("apply: %v", err)
		}
	}

	got := readServers(t, configPath)
	if len(got) != 1 {
		t.Fatalf("expected github pruned, got %v", got)
	}
	if _, ok := got["context7"]; !ok {
		t.Error("context7 should survive")
	}
}

// TestProfileSwitch_PrunesToEmpty covers the strict-empty filter: an active
// profile with mcp_servers: [] must remove every managed server.
func TestProfileSwitch_PrunesToEmpty(t *testing.T) {
	project := t.TempDir()
	full := map[string]domain.MCPServerDef{
		"context7": {Type: "local", Command: []string{"npx", "c7"}, Enabled: true},
	}
	configPath := installServers(t, project, full)

	adapter := opencode.New()
	inst := New(map[string]domain.MCPServerDef{}, Options{PruneWhenEmpty: true})
	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(actions) != 1 || actions[0].Action != domain.ActionUpdate {
		t.Fatalf("expected update action to prune, got %+v", actions)
	}
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("apply: %v", err)
		}
	}

	if got := readServers(t, configPath); len(got) != 0 {
		t.Errorf("expected all servers pruned, got %v", got)
	}

	// Idempotent: replanning against the pruned file is a skip.
	again, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("re-plan: %v", err)
	}
	if len(again) != 1 || again[0].Action != domain.ActionSkip {
		t.Fatalf("expected skip after prune, got %+v", again)
	}

	// Verify treats the pruned state as clean.
	results, err := inst.Verify(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed after prune: %s", r.Check, r.Message)
		}
	}
}

// TestEmptyServers_WithoutPruneOption_NoActions preserves legacy semantics:
// no configured servers and no profile filter means MCP is simply not
// managed — nothing is planned or deleted.
func TestEmptyServers_WithoutPruneOption_NoActions(t *testing.T) {
	project := t.TempDir()
	installServers(t, project, map[string]domain.MCPServerDef{
		"context7": {Type: "local", Command: []string{"npx", "c7"}, Enabled: true},
	})

	adapter := opencode.New()
	inst := New(map[string]domain.MCPServerDef{})
	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("expected no actions without the prune option, got %+v", actions)
	}
}

// TestPruneWhenEmpty_NoFileOnDisk_NoActions: strict-empty filter with nothing
// installed must not create an empty config file.
func TestPruneWhenEmpty_NoFileOnDisk_NoActions(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(map[string]domain.MCPServerDef{}, Options{PruneWhenEmpty: true})
	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("expected no actions with nothing on disk, got %+v", actions)
	}
	if _, err := os.Stat(adapter.ProjectConfigFile(project)); !os.IsNotExist(err) {
		t.Error("no config file should be created")
	}
}

// TestPrune_KeepsManagedKeysSidecarConsistent ensures the sidecar still lists
// the root key after a prune so future merges keep ownership.
func TestPrune_KeepsManagedKeysSidecarConsistent(t *testing.T) {
	project := t.TempDir()
	installServers(t, project, map[string]domain.MCPServerDef{
		"context7": {Type: "local", Command: []string{"npx", "c7"}, Enabled: true},
	})

	adapter := opencode.New()
	inst := New(map[string]domain.MCPServerDef{}, Options{PruneWhenEmpty: true})
	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("apply: %v", err)
		}
	}

	rel, err := filepath.Rel(project, adapter.ProjectConfigFile(project))
	if err != nil {
		t.Fatalf("rel: %v", err)
	}
	keys, err := managed.ReadManagedKeys(project, rel)
	if err != nil {
		t.Fatalf("read managed keys: %v", err)
	}
	found := false
	for _, k := range keys {
		if k == "mcp" {
			found = true
		}
	}
	if !found {
		t.Errorf("managed keys should still claim the mcp root key, got %v", keys)
	}
}
