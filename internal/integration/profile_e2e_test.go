package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/cli"
	"github.com/PedroMosquera/squadai/internal/components/memory"
	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/fileutil"
	"github.com/PedroMosquera/squadai/internal/marker"
	"github.com/PedroMosquera/squadai/internal/planner/budget"
)

// TestApplyProfileCheap_E2E runs `squadai apply --profile=cheap` end to end in
// a temp project: the plan must fit under the profile's 6k cap using a real
// tokenizer, the memory protocol must be the summary stub, and the MCP server
// list must be filtered down to the profile's (empty) allowlist.
func TestApplyProfileCheap_E2E(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	// Team-mode project with memory, MCP (two servers), skills, and agents.
	proj := domain.DefaultProjectConfig()
	proj.Adapters = map[string]domain.AdapterConfig{
		"opencode": {Enabled: true},
	}
	proj.Components = map[string]domain.ComponentConfig{
		string(domain.ComponentMemory): {Enabled: true},
		string(domain.ComponentMCP):    {Enabled: true},
		string(domain.ComponentSkills): {Enabled: true},
		string(domain.ComponentAgents): {Enabled: true},
	}
	proj.Methodology = domain.MethodologyTDD
	proj.Team = domain.DefaultTeam(domain.MethodologyTDD)
	proj.MCP = map[string]domain.MCPServerDef{
		"context7": {Type: "local", Command: []string{"npx", "-y", "@upstash/context7-mcp@latest"}, Enabled: true},
		"github":   {Type: "local", Command: []string{"npx", "-y", "@modelcontextprotocol/server-github"}, Enabled: true},
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), proj); err != nil {
		t.Fatalf("write project.json: %v", err)
	}

	var buf strings.Builder
	if err := cli.RunApply([]string{"--profile=cheap", "--agent=opencode", "--no-review"}, &buf); err != nil {
		t.Fatalf("apply --profile=cheap: %v\n%s", err, buf.String())
	}

	// 1. Budget sidecar: fit under the profile's 6k cap with a real model.
	fit, err := budget.Load(project)
	if err != nil {
		t.Fatalf("load applied budget: %v", err)
	}
	if fit == nil {
		t.Fatal("apply with a capped profile must persist .applied-budget.json")
	}
	if fit.Profile != "cheap" {
		t.Errorf("fit profile = %q, want cheap", fit.Profile)
	}
	if fit.Cap != 6000 {
		t.Errorf("fit cap = %d, want 6000", fit.Cap)
	}
	if !fit.FitAchieved {
		t.Errorf("fit not achieved: %+v", fit.Decisions)
	}
	if fit.TotalTokens > 6000 {
		t.Errorf("total tokens %d exceed the 6k cap", fit.TotalTokens)
	}
	if fit.Model == "" {
		t.Error("fit model must be resolved to a real tokenizer model")
	}

	// 2. Memory: the cheap profile's summary scope installs the stub.
	agentsMD, err := os.ReadFile(filepath.Join(project, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	memSection := marker.ExtractSection(string(agentsMD), memory.SectionIDForAgentID(domain.AgentOpenCode))
	if memSection != memory.ProtocolStub {
		t.Errorf("memory section should be the summary stub, got:\n%s", memSection)
	}

	// 3. MCP: the cheap profile allows no servers — nothing installed.
	adapter := opencode.New()
	if doc, err := fileutil.ReadJSONFile(adapter.ProjectConfigFile(project)); err == nil && doc != nil {
		if mcpVal, ok := doc["mcp"].(map[string]any); ok && len(mcpVal) > 0 {
			t.Errorf("cheap profile must filter out all MCP servers, found %v", mcpVal)
		}
	}

	// 4. Skills: only the shared scope is installed.
	skillsDir := adapter.ProjectSkillsDir(project)
	if _, err := os.Stat(filepath.Join(skillsDir, "shared", "code-review", "SKILL.md")); err != nil {
		t.Errorf("shared skill should be installed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "tdd")); !os.IsNotExist(err) {
		t.Error("tdd skills should be filtered out by the cheap profile's shared scope")
	}

	// 5. The always-on efficiency protocol is present.
	if !marker.HasSection(string(agentsMD), "efficiency:opencode") {
		t.Error("efficiency section missing from AGENTS.md")
	}

	// 6. token-budget --planned still reports the efficiency component as its
	// own row after apply.
	var tb strings.Builder
	if err := cli.RunTokenBudget([]string{"--planned"}, &tb); err != nil {
		t.Fatalf("token-budget --planned: %v", err)
	}
	if !strings.Contains(tb.String(), "efficiency") {
		t.Errorf("token-budget --planned should list the efficiency component:\n%s", tb.String())
	}
}

// TestApplyProfile_Unknown_E2E confirms an unknown --profile fails fast with
// the available names.
func TestApplyProfile_Unknown_E2E(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	t.Setenv("HOME", home)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(project); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	if err := config.WriteJSON(config.ProjectConfigPath(project), domain.DefaultProjectConfig()); err != nil {
		t.Fatalf("write project.json: %v", err)
	}

	var buf strings.Builder
	err = cli.RunApply([]string{"--profile=bogus", "--agent=opencode", "--no-review"}, &buf)
	if err == nil {
		t.Fatal("expected unknown-profile error")
	}
	if !strings.Contains(err.Error(), "unknown context profile") || !strings.Contains(err.Error(), "cheap") {
		t.Errorf("error should list available profiles, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(project, "AGENTS.md")); !os.IsNotExist(statErr) {
		t.Error("failed profile resolution must not write files")
	}
}

// marshal helper kept for symmetry with other integration files.
var _ = json.Marshal
