package mcp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/codex"
	"github.com/PedroMosquera/squadai/internal/adapters/cursor"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/pi"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/adapters/windsurf"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/managed"
)

var userServer = map[string]any{
	"command": "uvx",
	"args":    []any{"my-private-mcp", "--port", "9000"},
	"env":     map[string]any{"TOKEN": "${MY_TOKEN}"},
}

func ownershipAdapters() []domain.Adapter {
	return []domain.Adapter{claude.New(), cursor.New(), windsurf.New(), vscode.New(), pi.New(), opencode.New()}
}

func mcpTarget(adapter domain.Adapter, project string) string {
	if p := adapter.MCPConfigPath(project); p != "" {
		return p
	}
	return adapter.ProjectConfigFile(project)
}

func serversWith(url string, extra ...string) map[string]domain.MCPServerDef {
	out := map[string]domain.MCPServerDef{
		"context7": {Type: "remote", URL: url, Enabled: true},
	}
	for _, name := range extra {
		out[name] = domain.MCPServerDef{Type: "local", Command: []string{"npx", name}, Enabled: true}
	}
	return out
}

// rawServer returns the exact bytes of one server entry as stored on disk.
func rawServer(t *testing.T, path, rootKey, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var servers map[string]json.RawMessage
	if err := json.Unmarshal(doc[rootKey], &servers); err != nil {
		t.Fatalf("parse %s.%s: %v", path, rootKey, err)
	}
	return servers[name]
}

func serverNames(t *testing.T, path, rootKey string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	out := map[string]bool{}
	for name := range doc[rootKey] {
		out[name] = true
	}
	return out
}

// addUserServer rewrites the file with an extra server under rootKey, in the
// same layout squadai writes, and returns the exact bytes of that entry.
func addUserServer(t *testing.T, path, rootKey, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	servers, _ := doc[rootKey].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers[name] = userServer
	doc[rootKey] = servers
	writeTestJSON(t, path, doc)
	return rawServer(t, path, rootKey, name)
}

func TestOwnership_ReapplyKeepsUserAddedServer(t *testing.T) {
	for _, adapter := range ownershipAdapters() {
		t.Run(string(adapter.ID()), func(t *testing.T) {
			project := t.TempDir()
			planAndApply(t, New(serversWith("https://mcp.context7.com/mcp")), adapter, project)
			path := mcpTarget(adapter, project)
			rootKey := adapter.MCPRootKey()
			want := addUserServer(t, path, rootKey, "mine")

			planAndApply(t, New(serversWith("https://mcp.context7.com/mcp")), adapter, project)
			if got := rawServer(t, path, rootKey, "mine"); string(got) != string(want) {
				t.Fatalf("plain re-apply changed the user server:\nwant %s\ngot  %s", want, got)
			}

			// A real squadai change forces a write; the user server still survives.
			planAndApply(t, New(serversWith("https://mcp.context7.com/v2")), adapter, project)
			if got := rawServer(t, path, rootKey, "mine"); string(got) != string(want) {
				t.Fatalf("re-apply with a changed squadai server dropped or changed the user server:\nwant %s\ngot  %s", want, got)
			}
		})
	}
}

func TestOwnership_OverwriteConflictKeepsUserServers(t *testing.T) {
	for _, overwriteAll := range []bool{false, true} {
		name := "per-key"
		if overwriteAll {
			name = "overwrite-all"
		}
		t.Run(name, func(t *testing.T) {
			project := t.TempDir()
			adapter := claude.New()
			path := mcpTarget(adapter, project)
			writeTestJSON(t, path, map[string]any{
				"mcpServers": map[string]any{
					"mine":     userServer,
					"context7": map[string]any{"type": "http", "url": "https://user-fork.example.com/mcp"},
				},
			})
			want := rawServer(t, path, "mcpServers", "mine")

			inst := New(serversWith("https://mcp.context7.com/mcp"))
			entries, err := inst.Preview(adapter, t.TempDir(), project)
			if err != nil {
				t.Fatal(err)
			}
			var conflictKeys []string
			for _, e := range entries {
				for _, c := range e.Conflicts {
					conflictKeys = append(conflictKeys, c.Key)
				}
			}
			if len(conflictKeys) != 1 || conflictKeys[0] != "mcpServers.context7" {
				t.Fatalf("want a single conflict on mcpServers.context7, got %v", conflictKeys)
			}

			policy := domain.ApplyPolicy{OverwriteAll: overwriteAll}
			if !overwriteAll {
				policy.Overrides = map[string]map[string]bool{path: {conflictKeys[0]: true}}
			}
			inst.SetApplyPolicy(policy)
			planAndApply(t, inst, adapter, project)

			if got := rawServer(t, path, "mcpServers", "mine"); string(got) != string(want) {
				t.Fatalf("overwrite dropped or changed the user server:\nwant %s\ngot  %s", want, got)
			}
			var c7 map[string]any
			if err := json.Unmarshal(rawServer(t, path, "mcpServers", "context7"), &c7); err != nil {
				t.Fatal(err)
			}
			if c7["url"] != "https://mcp.context7.com/mcp" {
				t.Errorf("overwrite should replace context7 with squadai's value, got %v", c7)
			}
		})
	}
}

func TestOwnership_UnconsentedConflictBlocksOnlyThatServer(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	path := mcpTarget(adapter, project)
	writeTestJSON(t, path, map[string]any{
		"mcpServers": map[string]any{
			"context7": map[string]any{"type": "http", "url": "https://user-fork.example.com/mcp"},
		},
	})
	before, _ := os.ReadFile(path)

	inst := New(serversWith("https://mcp.context7.com/mcp"))
	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatal(err)
	}
	var conflict *domain.ConflictError
	for _, a := range actions {
		if err := inst.Apply(a); err != nil && !errors.As(err, &conflict) {
			t.Fatalf("apply: %v", err)
		}
	}
	if conflict == nil || len(conflict.Conflicts) != 1 || conflict.Conflicts[0].Key != "mcpServers.context7" {
		t.Fatalf("want ConflictError on mcpServers.context7, got %+v", conflict)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Error("a blocked apply must not write the file")
	}
}

func TestOwnership_RemovedSquadaiServerIsPrunedUserServersStay(t *testing.T) {
	for _, adapter := range ownershipAdapters() {
		t.Run(string(adapter.ID()), func(t *testing.T) {
			project := t.TempDir()
			planAndApply(t, New(serversWith("https://mcp.context7.com/mcp", "github")), adapter, project)
			path := mcpTarget(adapter, project)
			rootKey := adapter.MCPRootKey()
			want := addUserServer(t, path, rootKey, "mine")

			planAndApply(t, New(serversWith("https://mcp.context7.com/mcp")), adapter, project)

			names := serverNames(t, path, rootKey)
			if names["github"] {
				t.Error("github was removed from the squadai config and should be pruned")
			}
			if !names["context7"] {
				t.Error("context7 is still configured and should stay")
			}
			if got := rawServer(t, path, rootKey, "mine"); string(got) != string(want) {
				t.Fatalf("pruning dropped or changed the user server:\nwant %s\ngot  %s", want, got)
			}
		})
	}
}

func TestOwnership_VerifyCleanWithUserAddedServer(t *testing.T) {
	for _, adapter := range ownershipAdapters() {
		t.Run(string(adapter.ID()), func(t *testing.T) {
			project := t.TempDir()
			planAndApply(t, New(serversWith("https://mcp.context7.com/mcp")), adapter, project)
			addUserServer(t, mcpTarget(adapter, project), adapter.MCPRootKey(), "mine")

			inst := New(serversWith("https://mcp.context7.com/mcp"))
			assertVerifyPasses(t, inst, adapter, project)

			actions, err := inst.Plan(adapter, t.TempDir(), project)
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range actions {
				if a.Action != domain.ActionSkip {
					t.Errorf("a user-added server is not drift; want skip, got %s for %s", a.Action, a.TargetPath)
				}
			}
		})
	}
}

func TestOwnership_VerifyFlagsEditedSquadaiServer(t *testing.T) {
	for _, adapter := range ownershipAdapters() {
		t.Run(string(adapter.ID()), func(t *testing.T) {
			project := t.TempDir()
			planAndApply(t, New(serversWith("https://mcp.context7.com/mcp")), adapter, project)
			path := mcpTarget(adapter, project)
			rootKey := adapter.MCPRootKey()
			addUserServer(t, path, rootKey, "mine")

			data, _ := os.ReadFile(path)
			var doc map[string]any
			if err := json.Unmarshal(data, &doc); err != nil {
				t.Fatal(err)
			}
			servers := doc[rootKey].(map[string]any)
			servers["context7"].(map[string]any)[adapter.MCPURLKey()] = "https://hand-edited.example.com/mcp"
			writeTestJSON(t, path, doc)

			results, err := New(serversWith("https://mcp.context7.com/mcp")).Verify(adapter, t.TempDir(), project)
			if err != nil {
				t.Fatal(err)
			}
			failed := false
			for _, r := range results {
				if !r.Passed {
					failed = true
				}
			}
			if !failed {
				t.Errorf("hand-edited squadai server must be reported as drift, got %+v", results)
			}
		})
	}
}

func TestOwnership_SquadaiServerDeletedByUserIsDrift(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	planAndApply(t, New(serversWith("https://mcp.context7.com/mcp")), adapter, project)
	path := mcpTarget(adapter, project)
	writeTestJSON(t, path, map[string]any{"mcpServers": map[string]any{"mine": userServer}})

	results, err := New(serversWith("https://mcp.context7.com/mcp")).Verify(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatal(err)
	}
	failed := false
	for _, r := range results {
		if !r.Passed {
			failed = true
		}
	}
	if !failed {
		t.Error("a squadai server missing from the file must be reported as drift")
	}
}

// Sidecars written before per-server tracking record only the root key.
func TestOwnership_OldFormatMetadataMigrates(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	path := mcpTarget(adapter, project)
	writeTestJSON(t, path, map[string]any{
		"mcpServers": map[string]any{
			"context7":      map[string]any{"type": "http", "url": "https://stale.example.com/mcp"},
			"mine":          userServer,
			"dropped-early": map[string]any{"command": "npx", "args": []any{"old"}},
		},
	})
	if err := managed.WriteManagedKeys(project, ".mcp.json", []string{"mcpServers"}); err != nil {
		t.Fatal(err)
	}
	wantMine := rawServer(t, path, "mcpServers", "mine")
	wantDropped := rawServer(t, path, "mcpServers", "dropped-early")

	inst := New(serversWith("https://mcp.context7.com/mcp", "github"))
	planAndApply(t, inst, adapter, project)

	if got := rawServer(t, path, "mcpServers", "mine"); string(got) != string(wantMine) {
		t.Fatalf("migration dropped or changed the user server:\nwant %s\ngot  %s", wantMine, got)
	}
	if got := rawServer(t, path, "mcpServers", "dropped-early"); string(got) != string(wantDropped) {
		t.Fatalf("migration must never delete a server squadai does not configure:\nwant %s\ngot  %s", wantDropped, got)
	}
	var c7 map[string]any
	if err := json.Unmarshal(rawServer(t, path, "mcpServers", "context7"), &c7); err != nil {
		t.Fatal(err)
	}
	if c7["url"] != "https://mcp.context7.com/mcp" {
		t.Errorf("an owned key's configured server should be updated without a conflict, got %v", c7)
	}

	owned, tracked, err := managed.ReadManagedEntries(project, ".mcp.json", "mcpServers")
	if err != nil {
		t.Fatal(err)
	}
	if !tracked || len(owned) != 2 || owned[0] != "context7" || owned[1] != "github" {
		t.Errorf("after migration squadai should own exactly its configured servers, got %v (tracked=%v)", owned, tracked)
	}

	// Once tracked per server, removing github prunes it and nothing else.
	planAndApply(t, New(serversWith("https://mcp.context7.com/mcp")), adapter, project)
	names := serverNames(t, path, "mcpServers")
	if names["github"] || !names["mine"] || !names["dropped-early"] || !names["context7"] {
		t.Errorf("unexpected servers after pruning github: %v", names)
	}
}

// Codex already scopes ownership to its marker block; this pins that a user
// server table outside the block survives and is not drift.
func TestOwnership_CodexUserServerOutsideBlockSurvives(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := codex.New()
	path := filepath.Join(home, ".codex", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	user := "[mcp_servers.mine]\ncommand = \"uvx\"\nargs = [\"my-private-mcp\"]\n"
	if err := os.WriteFile(path, []byte(user), 0644); err != nil {
		t.Fatal(err)
	}

	for _, url := range []string{"https://mcp.context7.com/mcp", "https://mcp.context7.com/v2"} {
		inst := New(serversWith(url))
		actions, err := inst.Plan(adapter, home, project)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range actions {
			if err := inst.Apply(a); err != nil {
				t.Fatal(err)
			}
		}
	}

	data, _ := os.ReadFile(path)
	if !strings.HasPrefix(string(data), user) {
		t.Errorf("user server table changed:\n%s", data)
	}
	results, err := New(serversWith("https://mcp.context7.com/v2")).Verify(adapter, home, project)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("verify %s failed with a user server present: %s", r.Check, r.Message)
		}
	}
}

func TestOwnership_ClaudeAndVSCodeShareOwnershipOfRootMCPFile(t *testing.T) {
	project := t.TempDir()
	servers := serversWith("https://mcp.context7.com/mcp", "github")
	inst := New(servers)
	planAndApply(t, inst, claude.New(), project)
	planAndApply(t, inst, vscode.New(), project)
	path := filepath.Join(project, ".mcp.json")
	want := addUserServer(t, path, "mcpServers", "mine")

	next := New(serversWith("https://mcp.context7.com/mcp"))
	planAndApply(t, next, claude.New(), project)
	planAndApply(t, next, vscode.New(), project)

	names := serverNames(t, path, "mcpServers")
	if names["github"] || !names["context7"] {
		t.Errorf("unexpected servers: %v", names)
	}
	if got := rawServer(t, path, "mcpServers", "mine"); string(got) != string(want) {
		t.Fatalf("user server changed:\nwant %s\ngot  %s", want, got)
	}
}
