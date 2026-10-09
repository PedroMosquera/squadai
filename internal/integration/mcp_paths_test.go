package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/pi"
	"github.com/PedroMosquera/squadai/internal/adapters/vscode"
	"github.com/PedroMosquera/squadai/internal/cli"
	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/doctor"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/governance"
	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/pipeline"
	"github.com/PedroMosquera/squadai/internal/planner"
	"github.com/PedroMosquera/squadai/internal/verify"
)

// buildMCPConfig writes user and project config with memory + MCP enabled and
// each listed adapter set to the given state. OpenCode is disabled so verify
// does not depend on an installed opencode binary.
func buildMCPConfig(t *testing.T, home, project string, adapters map[domain.AgentID]bool) *domain.MergedConfig {
	t.Helper()
	userCfg := domain.DefaultUserConfig()
	userCfg.Adapters[string(domain.AgentOpenCode)] = domain.AdapterConfig{Enabled: false}
	projAdapters := make(map[string]domain.AdapterConfig, len(adapters))
	for id, enabled := range adapters {
		userCfg.Adapters[string(id)] = domain.AdapterConfig{Enabled: enabled}
		projAdapters[string(id)] = domain.AdapterConfig{Enabled: enabled}
	}
	if err := config.WriteJSON(config.UserConfigPath(home), userCfg); err != nil {
		t.Fatalf("write user config: %v", err)
	}
	projCfg := &domain.ProjectConfig{
		Version:  1,
		Adapters: projAdapters,
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory): {Enabled: true},
			string(domain.ComponentMCP):    {Enabled: true},
		},
		MCP: cli.DefaultMCPServers(),
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), projCfg); err != nil {
		t.Fatalf("write project config: %v", err)
	}
	user, err := config.LoadUser(home)
	if err != nil {
		t.Fatalf("load user: %v", err)
	}
	proj, err := config.LoadProject(project)
	if err != nil {
		t.Fatalf("load project: %v", err)
	}
	return config.Merge(user, proj, nil)
}

func planAll(t *testing.T, merged *domain.MergedConfig, adapters []domain.Adapter, home, project string) (*planner.Planner, []domain.PlannedAction) {
	t.Helper()
	p := planner.New()
	actions, err := p.Plan(merged, adapters, home, project)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	return p, actions
}

func applyAll(t *testing.T, merged *domain.MergedConfig, adapters []domain.Adapter, home, project string) {
	t.Helper()
	p, actions := planAll(t, merged, adapters, home, project)
	report, err := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot, nil).Execute(actions)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	for _, s := range report.Steps {
		if s.Status == domain.StepFailed {
			t.Errorf("apply step %q failed: %s", s.Action.ID, s.Error)
		}
	}
	if !report.Success {
		t.Fatal("apply should succeed")
	}
}

// assertConverged is the apply -> verify --strict -> doctor drift round trip,
// plus a re-plan that must hold no MCP work.
func assertConverged(t *testing.T, merged *domain.MergedConfig, adapters []domain.Adapter, home, project string) {
	t.Helper()
	vReport, err := verify.New().Verify(merged, adapters, home, project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, r := range vReport.Results {
		if !r.Passed {
			t.Errorf("verify check %q failed: %s", r.Check, r.Message)
		}
	}

	drifts, err := governance.CheckDrift(project)
	if err != nil {
		t.Fatalf("governance.CheckDrift: %v", err)
	}
	for _, r := range drifts {
		if r.Drifted() {
			t.Errorf("governance drift: %s: %s (%s)", r.Path, r.Detail, r.Kind)
		}
	}

	d := doctor.New(home, project, adapters, domain.DefaultMCPCatalog())
	results, err := d.Run(context.Background(), doctor.Options{Category: "drift"})
	if err != nil {
		t.Fatalf("doctor drift run: %v", err)
	}
	for _, r := range results {
		if r.Status == doctor.CheckFail {
			t.Errorf("doctor drift check %q failed: %s", r.Name, r.Message)
		}
	}

	_, actions := planAll(t, merged, adapters, home, project)
	for _, a := range actions {
		if a.Component == domain.ComponentMCP && a.Action != domain.ActionSkip {
			t.Errorf("MCP not converged, re-plan has %s %s on %s", a.ID, a.Action, a.TargetPath)
		}
		if a.Action == domain.ActionDelete {
			t.Errorf("re-plan deletes %s", a.TargetPath)
		}
	}
}

func readJSONMap(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return doc
}

func serversUnder(t *testing.T, path, rootKey string) map[string]any {
	t.Helper()
	servers, ok := readJSONMap(t, path)[rootKey].(map[string]any)
	if !ok {
		t.Fatalf("%s has no %q object", path, rootKey)
	}
	return servers
}

func TestFullPipeline_VSCode_MCPRoundTrip(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	merged := buildMCPConfig(t, home, project, map[domain.AgentID]bool{domain.AgentVSCodeCopilot: true})
	adapters := []domain.Adapter{vscode.New()}

	applyAll(t, merged, adapters, home, project)

	if _, ok := serversUnder(t, filepath.Join(project, ".mcp.json"), "mcpServers")["context7"]; !ok {
		t.Error(".mcp.json mcpServers missing context7")
	}
	if _, err := os.Stat(filepath.Join(project, ".vscode", "mcp.json")); !os.IsNotExist(err) {
		t.Errorf("fresh apply must not write the deprecated .vscode/mcp.json, stat err = %v", err)
	}
	assertConverged(t, merged, adapters, home, project)
}

func TestFullPipeline_VSCode_MigratesLegacyMCPKeepingUserServer(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	merged := buildMCPConfig(t, home, project, map[domain.AgentID]bool{domain.AgentVSCodeCopilot: true})
	adapters := []domain.Adapter{vscode.New()}

	legacyRel := filepath.Join(".vscode", "mcp.json")
	legacyPath := filepath.Join(project, legacyRel)
	if err := os.MkdirAll(filepath.Dir(legacyPath), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `{"servers": {"context7": {"type": "http", "url": "https://mcp.context7.com/mcp"}, "my-db": {"command": "pg-mcp"}}}`
	if err := os.WriteFile(legacyPath, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := managed.WriteManagedKeys(project, legacyRel, []string{"servers"}); err != nil {
		t.Fatal(err)
	}
	if err := managed.TrackCreatedFile(project, legacyRel); err != nil {
		t.Fatal(err)
	}

	applyAll(t, merged, adapters, home, project)

	if _, ok := serversUnder(t, filepath.Join(project, ".mcp.json"), "mcpServers")["context7"]; !ok {
		t.Error(".mcp.json mcpServers missing context7 after migration")
	}
	legacy := serversUnder(t, legacyPath, "servers")
	if _, ok := legacy["my-db"]; !ok {
		t.Errorf("user server my-db deleted from .vscode/mcp.json: %v", legacy)
	}
	if _, ok := legacy["context7"]; ok {
		t.Error("squadai context7 left in .vscode/mcp.json, VS Code would see it twice")
	}
	assertConverged(t, merged, adapters, home, project)
}

func TestFullPipeline_Pi_MigratesLegacyMCPOutOfPiJSON(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	merged := buildMCPConfig(t, home, project, map[domain.AgentID]bool{domain.AgentPi: true})
	adapters := []domain.Adapter{pi.New()}

	piJSON := filepath.Join(project, "pi.json")
	seed := `{"mcp": {"context7": {"type": "remote", "url": "https://mcp.context7.com/mcp"}, "mine": {"type": "local", "command": ["my-mcp"]}}}`
	if err := os.WriteFile(piJSON, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := managed.WriteManagedKeys(project, "pi.json", []string{"mcp"}); err != nil {
		t.Fatal(err)
	}

	applyAll(t, merged, adapters, home, project)

	if _, ok := serversUnder(t, filepath.Join(project, ".pi", "mcp.json"), "mcpServers")["context7"]; !ok {
		t.Error(".pi/mcp.json mcpServers missing context7")
	}
	legacy := serversUnder(t, piJSON, "mcp")
	if _, ok := legacy["mine"]; !ok {
		t.Errorf("user server deleted from pi.json: %v", legacy)
	}
	if _, ok := legacy["context7"]; ok {
		t.Error("squadai context7 left in pi.json")
	}
	assertConverged(t, merged, adapters, home, project)
}

func TestFullPipeline_ClaudeAndVSCode_ShareMCPFile(t *testing.T) {
	home, project := t.TempDir(), t.TempDir()
	adapters := []domain.Adapter{claude.New(), vscode.New()}
	both := buildMCPConfig(t, home, project, map[domain.AgentID]bool{
		domain.AgentClaudeCode: true, domain.AgentVSCodeCopilot: true,
	})

	applyAll(t, both, adapters, home, project)
	assertConverged(t, both, adapters, home, project)

	// Disabling either adapter must leave the shared file to the other one.
	for _, off := range []domain.AgentID{domain.AgentVSCodeCopilot, domain.AgentClaudeCode} {
		t.Run("disable "+string(off), func(t *testing.T) {
			states := map[domain.AgentID]bool{domain.AgentClaudeCode: true, domain.AgentVSCodeCopilot: true}
			states[off] = false
			merged := buildMCPConfig(t, home, project, states)
			_, actions := planAll(t, merged, adapters, home, project)
			for _, a := range actions {
				if a.TargetPath == filepath.Join(project, ".mcp.json") && a.Action != domain.ActionSkip {
					t.Errorf("disabling %s plans %s on the shared .mcp.json (%s)", off, a.Action, a.ID)
				}
			}
			applyAll(t, merged, adapters, home, project)
			if _, ok := serversUnder(t, filepath.Join(project, ".mcp.json"), "mcpServers")["context7"]; !ok {
				t.Error("shared .mcp.json lost context7")
			}
			// Only .mcp.json is checked for drift: stale cleanup of the
			// disabled adapter's own files leaves sidecar entries behind,
			// which is unrelated to the shared MCP file.
			assertMCPFileConverged(t, merged, adapters, home, project)
		})
	}
}

func assertMCPFileConverged(t *testing.T, merged *domain.MergedConfig, adapters []domain.Adapter, home, project string) {
	t.Helper()
	vReport, err := verify.New().Verify(merged, adapters, home, project)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, r := range vReport.Results {
		if !r.Passed && r.Component == "mcp" {
			t.Errorf("verify check %q failed: %s", r.Check, r.Message)
		}
	}
	drifts, err := governance.CheckDrift(project)
	if err != nil {
		t.Fatalf("governance.CheckDrift: %v", err)
	}
	for _, r := range drifts {
		if r.Path == ".mcp.json" && r.Drifted() {
			t.Errorf("governance drift on .mcp.json: %s (%s)", r.Detail, r.Kind)
		}
	}
	_, actions := planAll(t, merged, adapters, home, project)
	for _, a := range actions {
		if a.Component == domain.ComponentMCP && a.Action != domain.ActionSkip {
			t.Errorf("MCP not converged, re-plan has %s %s on %s", a.ID, a.Action, a.TargetPath)
		}
	}
}
