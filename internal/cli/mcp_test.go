package cli

import (
	"context"
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// ─── Mock adapter for filter tests ──────────────────────────────────────────

type mcpTestAdapter struct {
	id domain.AgentID
}

func (a mcpTestAdapter) ID() domain.AgentID       { return a.id }
func (a mcpTestAdapter) Lane() domain.AdapterLane { return domain.LanePersonal }
func (a mcpTestAdapter) Detect(_ context.Context, _ string) (bool, bool, error) {
	return true, true, nil
}
func (a mcpTestAdapter) GlobalConfigDir(_ string) string             { return "" }
func (a mcpTestAdapter) SystemPromptFile(_ string) string            { return "" }
func (a mcpTestAdapter) SkillsDir(_ string) string                   { return "" }
func (a mcpTestAdapter) SettingsPath(_ string) string                { return "" }
func (a mcpTestAdapter) SupportsComponent(_ domain.ComponentID) bool { return false }
func (a mcpTestAdapter) ProjectConfigFile(_ string) string           { return "" }
func (a mcpTestAdapter) ProjectRulesFile(_ string) string            { return "" }
func (a mcpTestAdapter) ProjectAgentsDir(_ string) string            { return "" }
func (a mcpTestAdapter) ProjectSkillsDir(_ string) string            { return "" }
func (a mcpTestAdapter) ProjectCommandsDir(_ string) string          { return "" }
func (a mcpTestAdapter) DelegationStrategy() domain.DelegationStrategy {
	return domain.DelegationSoloAgent
}
func (a mcpTestAdapter) SupportsSubAgents() bool       { return false }
func (a mcpTestAdapter) SubAgentsDir(_ string) string  { return "" }
func (a mcpTestAdapter) SupportsWorkflows() bool       { return false }
func (a mcpTestAdapter) WorkflowsDir(_ string) string  { return "" }
func (a mcpTestAdapter) MCPRootKey() string                        { return "mcpServers" }
func (a mcpTestAdapter) MCPURLKey() string                         { return "url" }
func (a mcpTestAdapter) MCPConfigPath(_ string) string             { return "" }
func (a mcpTestAdapter) MCPCommandStyle() string                   { return "split" }
func (a mcpTestAdapter) MCPEnvKey() string                         { return "env" }
func (a mcpTestAdapter) MCPTypeField(_ domain.MCPServerDef) string { return "" }
func (a mcpTestAdapter) RulesFrontmatter() string                  { return "" }

// ─── DefaultMCPServers ───────────────────────────────────────────────────────

func TestDefaultMCPServers_MatchesCatalogCount(t *testing.T) {
	servers := DefaultMCPServers()
	catalog := domain.DefaultMCPCatalog()
	if len(servers) != len(catalog) {
		t.Errorf("DefaultMCPServers len = %d, want %d (must match catalog)", len(servers), len(catalog))
	}
}

func TestDefaultMCPServers_AllCatalogEntriesPresent(t *testing.T) {
	servers := DefaultMCPServers()
	for _, entry := range domain.DefaultMCPCatalog() {
		if _, ok := servers[entry.Name]; !ok {
			t.Errorf("DefaultMCPServers missing catalog entry %q", entry.Name)
		}
	}
}

func TestDefaultMCPServers_HasContext7(t *testing.T) {
	servers := DefaultMCPServers()
	if _, ok := servers["context7"]; !ok {
		t.Error("DefaultMCPServers should contain 'context7' key")
	}
}

func TestDefaultMCPServers_Context7Enabled(t *testing.T) {
	servers := DefaultMCPServers()
	c7, ok := servers["context7"]
	if !ok {
		t.Fatal("context7 not found in DefaultMCPServers")
	}
	if !c7.Enabled {
		t.Error("context7 should be Enabled by default")
	}
}

func TestDefaultMCPServers_Context7HasCommand(t *testing.T) {
	servers := DefaultMCPServers()
	c7, ok := servers["context7"]
	if !ok {
		t.Fatal("context7 not found in DefaultMCPServers")
	}
	if len(c7.Command) == 0 {
		t.Error("context7 Command should be non-empty")
	}
	found := false
	for _, part := range c7.Command {
		if part == "npx" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("context7 Command should contain 'npx', got %v", c7.Command)
	}
}

// ─── AvailablePlugins ────────────────────────────────────────────────────────

func TestAvailablePlugins_HasFourPlugins(t *testing.T) {
	plugins := AvailablePlugins()
	if len(plugins) != 4 {
		t.Errorf("AvailablePlugins len = %d, want 4", len(plugins))
	}
}

func TestAvailablePlugins_SuperpowersExcludesTDD(t *testing.T) {
	plugins := AvailablePlugins()
	sp, ok := plugins["superpowers"]
	if !ok {
		t.Fatal("superpowers not found in AvailablePlugins")
	}
	if sp.ExcludesMethodology != "tdd" {
		t.Errorf("superpowers ExcludesMethodology = %q, want %q", sp.ExcludesMethodology, "tdd")
	}
}

// ─── FilterPlugins ───────────────────────────────────────────────────────────

func TestFilterPlugins_TDD_ExcludesSuperpowers(t *testing.T) {
	adapters := []domain.Adapter{
		mcpTestAdapter{id: domain.AgentClaudeCode},
	}
	filtered := FilterPlugins(AvailablePlugins(), adapters, domain.MethodologyTDD)
	if _, ok := filtered["superpowers"]; ok {
		t.Error("superpowers should be excluded when methodology is TDD")
	}
}

func TestFilterPlugins_ClaudeDetected_HasCodeSimplifier(t *testing.T) {
	adapters := []domain.Adapter{
		mcpTestAdapter{id: domain.AgentClaudeCode},
	}
	filtered := FilterPlugins(AvailablePlugins(), adapters, domain.MethodologyConventional)
	if _, ok := filtered["code-simplifier"]; !ok {
		t.Error("code-simplifier should be included when claude-code is detected")
	}
}

func TestFilterPlugins_NoAgents_EmptyResult(t *testing.T) {
	filtered := FilterPlugins(AvailablePlugins(), nil, domain.MethodologyConventional)
	if len(filtered) != 0 {
		t.Errorf("FilterPlugins with no agents should return empty, got %d plugins", len(filtered))
	}
}
