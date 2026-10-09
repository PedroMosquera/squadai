package mcp

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/pi"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/managed"
)

var legacyVSCodeRel = filepath.Join(".vscode", "mcp.json")

func planAndApply(t *testing.T, inst *Installer, adapter domain.Adapter, project string) []domain.PlannedAction {
	t.Helper()
	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("plan %s: %v", adapter.ID(), err)
	}
	for _, a := range actions {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("apply %s: %v", a.ID, err)
		}
	}
	return actions
}

func hasActionFor(actions []domain.PlannedAction, path string, kind domain.ActionType) bool {
	for _, a := range actions {
		if a.TargetPath == path && a.Action == kind {
			return true
		}
	}
	return false
}

func assertVerifyPasses(t *testing.T, inst *Installer, adapter domain.Adapter, project string) {
	t.Helper()
	results, err := inst.Verify(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("verify %s failed: %s", r.Check, r.Message)
		}
	}
}

// seedLegacyVSCode writes the pre-migration state: squadai owns the "servers"
// key (sidecar) and the user has since added their own server inside it.
func seedLegacyVSCode(t *testing.T, project string, servers map[string]any, extra map[string]any) string {
	t.Helper()
	path := filepath.Join(project, legacyVSCodeRel)
	doc := map[string]any{"servers": servers}
	for k, v := range extra {
		doc[k] = v
	}
	writeTestJSON(t, path, doc)
	if err := managed.WriteManagedKeys(project, legacyVSCodeRel, []string{"servers"}); err != nil {
		t.Fatal(err)
	}
	return path
}

var squadaiContext7VSCode = map[string]any{"type": "http", "url": "https://mcp.context7.com/mcp"}

func TestLegacyMigration_VSCode_UserServerSurvivesSquadaiServerMoves(t *testing.T) {
	project := t.TempDir()
	userServer := map[string]any{"command": "pg-mcp", "args": []any{"--ro"}}
	inputs := []any{map[string]any{"id": "pg-pass", "type": "promptString"}}
	legacyPath := seedLegacyVSCode(t, project,
		map[string]any{"context7": squadaiContext7VSCode, "my-db": userServer},
		map[string]any{"inputs": inputs})

	adapter := vscode.New()
	inst := newTestInstaller()
	planAndApply(t, inst, adapter, project)

	newDoc := readTestJSON(t, filepath.Join(project, ".mcp.json"))
	newServers, _ := newDoc["mcpServers"].(map[string]any)
	if _, ok := newServers["context7"]; !ok {
		t.Fatalf(".mcp.json mcpServers missing context7: %v", newDoc)
	}
	if _, ok := newServers["my-db"]; ok {
		t.Error("user server must stay in the file the user put it in, not be copied into .mcp.json")
	}

	legacy := readTestJSON(t, legacyPath)
	legacyServers, _ := legacy["servers"].(map[string]any)
	if _, ok := legacyServers["context7"]; ok {
		t.Error("squadai-owned context7 must be removed from .vscode/mcp.json to avoid a duplicate")
	}
	if _, ok := legacyServers["my-db"]; !ok {
		t.Errorf("user-added my-db was deleted from .vscode/mcp.json: %v", legacy)
	}
	if _, ok := legacy["inputs"]; !ok {
		t.Error("user inputs array was dropped from .vscode/mcp.json")
	}

	keys, err := managed.ReadManagedKeys(project, legacyVSCodeRel)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Errorf("squadai must stop owning the legacy servers key, still manages %v", keys)
	}

	// Converges: the next plan neither rewrites .mcp.json nor touches the legacy file.
	second, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range second {
		if a.Action != domain.ActionSkip {
			t.Errorf("second plan not converged: %s %s %s", a.ID, a.Action, a.TargetPath)
		}
	}
	assertVerifyPasses(t, inst, adapter, project)
}

func TestLegacyMigration_VSCode_SquadaiOnlyLegacyFileIsRemoved(t *testing.T) {
	project := t.TempDir()
	legacyPath := seedLegacyVSCode(t, project, map[string]any{"context7": squadaiContext7VSCode}, nil)
	if err := managed.TrackCreatedFile(project, legacyVSCodeRel); err != nil {
		t.Fatal(err)
	}

	planAndApply(t, newTestInstaller(), vscode.New(), project)

	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Errorf("legacy file holding only squadai content should be removed, stat err = %v", err)
	}
	files, _ := managed.ListManagedFiles(project)
	for _, f := range files {
		if f == legacyVSCodeRel {
			t.Error("removed legacy file still listed as managed; drift would report it deleted")
		}
	}
	created, _ := managed.ListCreatedFiles(project)
	for _, f := range created {
		if f == legacyVSCodeRel {
			t.Error("removed legacy file still tracked as created; drift would report it deleted")
		}
	}
}

func TestLegacyMigration_VSCode_UnmanagedLegacyFileUntouched(t *testing.T) {
	project := t.TempDir()
	legacyPath := filepath.Join(project, legacyVSCodeRel)
	writeTestJSON(t, legacyPath, map[string]any{"servers": map[string]any{"context7": squadaiContext7VSCode}})
	before, _ := os.ReadFile(legacyPath)

	actions := planAndApply(t, newTestInstaller(), vscode.New(), project)

	if hasActionFor(actions, legacyPath, domain.ActionUpdate) {
		t.Error("squadai never owned this servers key and must not plan changes to it")
	}
	after, _ := os.ReadFile(legacyPath)
	if !bytes.Equal(before, after) {
		t.Errorf("user-owned .vscode/mcp.json changed:\nbefore %s\nafter  %s", before, after)
	}
}

func TestLegacyMigration_VSCode_LegacyKeptWhenNewFileBlocked(t *testing.T) {
	project := t.TempDir()
	legacyPath := seedLegacyVSCode(t, project, map[string]any{"context7": squadaiContext7VSCode}, nil)
	before, _ := os.ReadFile(legacyPath)
	// A user-owned mcpServers key in .mcp.json blocks squadai's write.
	writeTestJSON(t, filepath.Join(project, ".mcp.json"),
		map[string]any{"mcpServers": map[string]any{"theirs": map[string]any{"command": "x"}}})

	inst := newTestInstaller()
	actions, err := inst.Plan(vscode.New(), t.TempDir(), project)
	if err != nil {
		t.Fatal(err)
	}
	var sawConflict bool
	for _, a := range actions {
		err := inst.Apply(a)
		var ce *domain.ConflictError
		if errors.As(err, &ce) {
			sawConflict = true
		}
	}
	if !sawConflict {
		t.Fatal("precondition: expected the .mcp.json write to conflict")
	}
	after, _ := os.ReadFile(legacyPath)
	if !bytes.Equal(before, after) {
		t.Error("legacy servers were removed even though .mcp.json was not written; VS Code would lose them")
	}
}

func TestLegacyMigration_VSCode_VerifyFlagsPendingMigration(t *testing.T) {
	project := t.TempDir()
	seedLegacyVSCode(t, project, map[string]any{"context7": squadaiContext7VSCode}, nil)

	inst := newTestInstaller()
	results, err := inst.Verify(vscode.New(), t.TempDir(), project)
	if err != nil {
		t.Fatal(err)
	}
	var flagged bool
	for _, r := range results {
		if r.Check == "mcp-legacy-migrated" && !r.Passed {
			flagged = true
		}
	}
	if !flagged {
		t.Errorf("verify must flag squadai servers left in .vscode/mcp.json, got %+v", results)
	}
}

func TestLegacyMigration_VSCode_PreviewReportsNoConflictForLegacyRewrite(t *testing.T) {
	project := t.TempDir()
	legacyPath := seedLegacyVSCode(t, project, map[string]any{
		"context7": squadaiContext7VSCode,
		"my-db":    map[string]any{"command": "pg-mcp"},
	}, nil)

	entries, err := newTestInstaller().Preview(vscode.New(), t.TempDir(), project)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, e := range entries {
		if e.TargetPath != legacyPath {
			continue
		}
		found = true
		if len(e.Conflicts) != 0 {
			t.Errorf("legacy rewrite reported conflicts: %+v", e.Conflicts)
		}
		if !bytes.Contains([]byte(e.Diff), []byte("context7")) {
			t.Errorf("legacy diff should show context7 being removed, got:\n%s", e.Diff)
		}
	}
	if !found {
		t.Error("preview has no entry for the legacy file")
	}
}

func TestLegacyMigration_Pi_MovesServersOutOfPiJSONKeepingOtherKeys(t *testing.T) {
	project := t.TempDir()
	piJSON := filepath.Join(project, "pi.json")
	writeTestJSON(t, piJSON, map[string]any{
		"mcp": map[string]any{
			"context7": map[string]any{"type": "remote", "url": "https://mcp.context7.com/mcp"},
			"mine":     map[string]any{"type": "local", "command": []any{"my-mcp"}},
		},
		"permission": map[string]any{"bash": "ask"},
	})
	if err := managed.WriteManagedKeys(project, "pi.json", []string{"mcp", "permission"}); err != nil {
		t.Fatal(err)
	}

	adapter := pi.New()
	inst := newTestInstaller()
	planAndApply(t, inst, adapter, project)

	newDoc := readTestJSON(t, filepath.Join(project, ".pi", "mcp.json"))
	servers, _ := newDoc["mcpServers"].(map[string]any)
	got, _ := servers["context7"].(map[string]any)
	if got["url"] != "https://mcp.context7.com/mcp" || got["type"] != "http" {
		t.Errorf(".pi/mcp.json context7 = %v, want url entry with type http", got)
	}

	legacy := readTestJSON(t, piJSON)
	if _, ok := legacy["permission"]; !ok {
		t.Error("unrelated pi.json key was dropped")
	}
	legacyServers, _ := legacy["mcp"].(map[string]any)
	if _, ok := legacyServers["context7"]; ok {
		t.Error("squadai-owned context7 must leave pi.json")
	}
	if _, ok := legacyServers["mine"]; !ok {
		t.Errorf("user-added server deleted from pi.json: %v", legacy)
	}
	keys, _ := managed.ReadManagedKeys(project, "pi.json")
	if len(keys) != 1 || keys[0] != "permission" {
		t.Errorf("pi.json managed keys = %v, want [permission]", keys)
	}
	assertVerifyPasses(t, inst, adapter, project)
}

func TestSharedMCPFile_ClaudeAndVSCodeConverge(t *testing.T) {
	project := t.TempDir()
	inst := New(map[string]domain.MCPServerDef{
		"context7": {Type: "remote", URL: "https://mcp.context7.com/mcp", Headers: map[string]string{"X": "y"}, Enabled: true},
		"squadai":  {Type: "local", Command: []string{"squadai", "mcp-server"}, Environment: map[string]string{"A": "b"}, Enabled: true},
	})
	cc, vs := claude.New(), vscode.New()
	if cc.MCPConfigPath(project) != vs.MCPConfigPath(project) {
		t.Fatalf("precondition: adapters should share one file, got %s and %s",
			cc.MCPConfigPath(project), vs.MCPConfigPath(project))
	}

	ccActions, err := inst.Plan(cc, t.TempDir(), project)
	if err != nil {
		t.Fatal(err)
	}
	vsActions, err := inst.Plan(vs, t.TempDir(), project)
	if err != nil {
		t.Fatal(err)
	}
	ccRender, err := inst.RenderContent(ccActions[0])
	if err != nil {
		t.Fatal(err)
	}
	vsRender, err := inst.RenderContent(vsActions[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ccRender, vsRender) {
		t.Fatalf("claude and vscode serialize the shared .mcp.json differently, applies would flap:\nclaude %s\nvscode %s", ccRender, vsRender)
	}

	for _, a := range append(ccActions, vsActions...) {
		if err := inst.Apply(a); err != nil {
			t.Fatalf("apply %s: %v", a.ID, err)
		}
	}
	for _, ad := range []domain.Adapter{cc, vs} {
		actions, err := inst.Plan(ad, t.TempDir(), project)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range actions {
			if a.Action != domain.ActionSkip {
				t.Errorf("%s not converged after shared apply: %s %s", ad.ID(), a.ID, a.Action)
			}
		}
		assertVerifyPasses(t, inst, ad, project)
	}
}
