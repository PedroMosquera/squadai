package config

import (
	"reflect"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/codex"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/model"
)

func allowListAdapters() []domain.Adapter {
	return []domain.Adapter{claude.New(), codex.New()}
}

func TestCheckAllowLists_TeamRoleResolvedModel(t *testing.T) {
	cfg := &domain.MergedConfig{
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentClaudeCode): {Enabled: true},
			string(domain.AgentCodex):      {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{string(domain.ComponentAgents): {Enabled: true}},
		Team:       map[string]domain.TeamRole{"implementer": {Model: "premium"}},
	}
	premium := model.ForAgent(domain.AgentClaudeCode).Resolve(model.TierPremium)
	cfg.Allowed = domain.AllowLists{Models: []string{premium}}
	if got := CheckAllowLists(cfg, allowListAdapters(), nil); len(got) != 0 {
		t.Fatalf("role resolving to an allowed model must pass, got %v", got)
	}

	cfg.Allowed = domain.AllowLists{Models: []string{"no-such-model"}}
	got := CheckAllowLists(cfg, allowListAdapters(), nil)
	if len(got) != 1 || !strings.Contains(got[0], `team role "implementer"`) || !strings.Contains(got[0], premium) {
		t.Errorf("want one violation for the claude role only (codex writes no agents), got %v", got)
	}

	cfg.Components = nil
	if got := CheckAllowLists(cfg, allowListAdapters(), nil); len(got) != 0 {
		t.Errorf("roles are not written without the agents component, got %v", got)
	}
}

func TestCheckAllowLists_MCPHooksAndPlugins(t *testing.T) {
	cfg := &domain.MergedConfig{
		MCP: map[string]domain.MCPServerDef{
			"local":    {Type: "local", Command: []string{"npx", "-y", "vetted-mcp@1.0.0"}, Enabled: true},
			"remote":   {Type: "remote", URL: "https://mcp.example.invalid/sse", Enabled: true},
			"disabled": {Type: "local", Command: []string{"curl"}, Enabled: false},
		},
		Hooks: domain.HooksConfig{
			"Stop": {{Hooks: []domain.HookEntry{{Type: "command", Command: "squadai budget check"}, {Type: "command", Command: "bash run.sh"}}}},
		},
		Plugins: map[string]domain.PluginDef{
			"vetted":  {Enabled: true, PluginID: "vetted@official-market"},
			"stray":   {Enabled: true, PluginID: "stray@someone-else"},
			"bundled": {Enabled: true, InstallMethod: "skill_files"},
			"rehomed": {Enabled: true, PluginID: "rehomed@official-market",
				Marketplace: &domain.PluginMarketplace{Source: "github", Repo: "someone/official-market"}},
			"pinned": {Enabled: true, PluginID: "pinned@official-market",
				Marketplace: &domain.PluginMarketplace{Source: "github", Repo: "vetted-org/official-market"}},
		},
		Allowed: domain.AllowLists{
			MCPServers:    &domain.MCPServerAllowList{Commands: []string{"npx -y vetted-mcp@*"}, URLs: []string{"https://internal.invalid/*"}},
			HookCommands:  []string{"squadai *"},
			PluginSources: []string{"official-market", "github.com/vetted-org/*"},
		},
	}
	want := []string{
		`MCP server "remote" URL "https://mcp.example.invalid/sse" is not in allowed.mcp_servers.urls`,
		`Stop hook command "bash run.sh" is not in allowed.hook_commands`,
		`plugin "rehomed" marketplace "official-market" is fetched from "github.com/someone/official-market", which is not in allowed.plugin_sources`,
		`plugin "stray" comes from "someone-else", which is not in allowed.plugin_sources`,
	}
	if got := CheckAllowLists(cfg, nil, nil); !reflect.DeepEqual(got, want) {
		t.Errorf("violations:\n got %q\nwant %q", got, want)
	}
}

func TestCheckAllowLists_PackageSources(t *testing.T) {
	cfg := &domain.MergedConfig{
		Plugins: map[string]domain.PluginDef{
			"scoped":    {Enabled: true, InstallMethod: "package", Package: "npm:@vetted-org/tools@1.2.3"},
			"stray":     {Enabled: true, InstallMethod: "package", Package: "npm:stray-tools@1.0.0"},
			"repo":      {Enabled: true, InstallMethod: "package", Package: "git:github.com/vetted-org/pi-ext@v1"},
			"elsewhere": {Enabled: true, InstallMethod: "package", Package: "git:gitlab.example.invalid/someone/ext@v2"},
			"off":       {Enabled: false, InstallMethod: "package", Package: "npm:off-tools@1.0.0"},
		},
		Allowed: domain.AllowLists{PluginSources: []string{"npm:@vetted-org/*", "github.com/vetted-org/*"}},
	}
	want := []string{
		`plugin "elsewhere" package comes from "gitlab.example.invalid/someone/ext", which is not in allowed.plugin_sources`,
		`plugin "stray" package comes from "npm:stray-tools", which is not in allowed.plugin_sources`,
	}
	if got := CheckAllowLists(cfg, nil, nil); !reflect.DeepEqual(got, want) {
		t.Errorf("violations:\n got %q\nwant %q", got, want)
	}
}

func TestCheckAllowLists_EmptyListAllowsNothing(t *testing.T) {
	cfg := &domain.MergedConfig{
		Agents:  map[string]domain.AgentDef{"a": {Model: "any-model"}},
		Allowed: domain.AllowLists{Models: []string{}},
	}
	if got := CheckAllowLists(cfg, nil, nil); len(got) != 1 {
		t.Errorf("a present but empty list must deny, got %v", got)
	}
	cfg.Allowed = domain.AllowLists{}
	if got := CheckAllowLists(cfg, nil, nil); len(got) != 0 {
		t.Errorf("an absent list must not restrict, got %v", got)
	}
}

func TestMerge_AllowedModelsEmitClaudeAvailableModels(t *testing.T) {
	claudeID := string(domain.AgentClaudeCode)
	project := &domain.ProjectConfig{Adapters: map[string]domain.AdapterConfig{claudeID: {Enabled: true}}}
	policyWith := func(models []string) *domain.PolicyConfig {
		return &domain.PolicyConfig{Version: 1, Mode: domain.ModeTeam, Allowed: domain.AllowLists{Models: models}}
	}

	got := Merge(nil, project, policyWith([]string{"claude-sonnet-5-*", "claude-haiku-4-5", "anthropic/claude-*"})).
		Adapters[claudeID].Settings["availableModels"]
	if want := []string{"claude-sonnet-5", "claude-haiku-4-5"}; !reflect.DeepEqual(got, want) {
		t.Errorf("availableModels = %#v, want %#v", got, want)
	}

	for name, models := range map[string][]string{
		"absent list":           nil,
		"untranslatable glob":   {"claude-sonnet-5-*", "*haiku*"},
		"provider IDs only":     {"anthropic/claude-*"},
		"wildcard mid-segment":  {"claude-sonnet-5*"},
		"single-char wildcards": {"claude-haiku-4-?"},
	} {
		settings := Merge(nil, project, policyWith(models)).Adapters[claudeID].Settings
		if _, ok := settings["availableModels"]; ok {
			t.Errorf("%s: availableModels must not be emitted when the list cannot be expressed exactly, got %v", name, settings)
		}
	}

	explicit := policyWith([]string{"claude-sonnet-5-*"})
	explicit.Required.Adapters = map[string]domain.AdapterConfig{claudeID: {Enabled: true, Settings: map[string]interface{}{"availableModels": []string{"sonnet"}}}}
	if got := Merge(nil, project, explicit).Adapters[claudeID].Settings["availableModels"]; !reflect.DeepEqual(got, []string{"sonnet"}) {
		t.Errorf("an explicit required availableModels must win, got %#v", got)
	}
}

func TestValidatePolicy_EmptyAllowListPattern(t *testing.T) {
	cfg := domain.DefaultPolicyConfig()
	cfg.Allowed = domain.AllowLists{MCPServers: &domain.MCPServerAllowList{Names: []string{""}}}
	issues := ValidatePolicy(cfg)
	if len(issues) != 1 || issues[0] != "allowed.mcp_servers.names contains an empty pattern" {
		t.Errorf("issues = %v", issues)
	}
}
