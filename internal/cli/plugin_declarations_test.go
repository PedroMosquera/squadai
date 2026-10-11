package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
)

func declareTeamPlugin(proj *domain.ProjectConfig) {
	proj.Components[string(domain.ComponentPlugins)] = domain.ComponentConfig{Enabled: true}
	proj.Plugins = map[string]domain.PluginDef{
		"formatter": {
			Enabled:         true,
			SupportedAgents: []string{"claude-code", "opencode"},
			InstallMethod:   "claude_plugin",
			PluginID:        "formatter@team-tools",
			Marketplace:     &domain.PluginMarketplace{Source: "github", Repo: "example-org/claude-plugins"},
		},
	}
}

func TestVerifyCI_CleanAfterApplyingPluginDeclarations(t *testing.T) {
	project := setupCIProjectWith(t, declareTeamPlugin)

	data, err := os.ReadFile(filepath.Join(project, ".claude", "settings.json"))
	if err != nil {
		t.Fatalf("apply should write the project's Claude settings: %v", err)
	}
	var doc struct {
		EnabledPlugins         map[string]bool `json:"enabledPlugins"`
		ExtraKnownMarketplaces map[string]any  `json:"extraKnownMarketplaces"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if !doc.EnabledPlugins["formatter@team-tools"] || doc.ExtraKnownMarketplaces["team-tools"] == nil {
		t.Fatalf("project settings should enable the plugin and declare its marketplace, got:\n%s", data)
	}

	out, err := runVerifyCIForTest(t)
	if err != nil {
		t.Fatalf("verify --ci should be clean right after apply, got %v\n%s", err, out)
	}
}

func TestVerifyCI_PluginMarketplaceOutsideAllowListIsAPolicyViolation(t *testing.T) {
	project := setupCIProjectWith(t, declareTeamPlugin)
	writeCIPolicy(t, project, domain.AllowLists{
		MCPServers:    &domain.MCPServerAllowList{Names: []string{"docs"}, URLs: []string{"https://example.invalid/*"}},
		PluginSources: []string{"team-tools"},
	})

	out, err := runVerifyCIForTest(t)
	requirePolicyViolation(t, err, out)
	if !strings.Contains(out, "github.com/example-org/claude-plugins") {
		t.Errorf("output should name the marketplace repository, got:\n%s", out)
	}

	writeCIPolicy(t, project, domain.AllowLists{
		MCPServers:    &domain.MCPServerAllowList{Names: []string{"docs"}, URLs: []string{"https://example.invalid/*"}},
		PluginSources: []string{"team-tools", "github.com/example-org/*"},
	})
	if out, err := runVerifyCIForTest(t); err != nil {
		t.Errorf("an allowed marketplace and repository must pass, got %v\n%s", err, out)
	}
}

func TestValidatePolicy_RejectsDeclaredPluginFromUnlistedMarketplace(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	proj := allowListProject()
	declareTeamPlugin(proj)
	writeProjectFile(t, project, proj)
	pol := domain.DefaultPolicyConfig()
	pol.Allowed = domain.AllowLists{PluginSources: []string{"claude-plugins-official", "github.com/example-org/*"}}
	writePolicyFile(t, project, pol)

	var buf strings.Builder
	err := RunValidatePolicy(nil, &buf)
	requirePolicyViolation(t, err, buf.String())
	if !strings.Contains(buf.String(), "team-tools") {
		t.Errorf("output should name the marketplace, got:\n%s", buf.String())
	}
}

func declarePackagePlugins(proj *domain.ProjectConfig) {
	proj.Adapters[string(domain.AgentPi)] = domain.AdapterConfig{Enabled: true}
	proj.Components[string(domain.ComponentPlugins)] = domain.ComponentConfig{Enabled: true}
	proj.Plugins = map[string]domain.PluginDef{
		"tools": {
			Enabled:         true,
			SupportedAgents: []string{"pi", "opencode"},
			InstallMethod:   "package",
			Package:         "npm:@example-org/agent-tools@1.2.3",
		},
		"pi-ext": {
			Enabled:         true,
			SupportedAgents: []string{"pi"},
			InstallMethod:   "package",
			Package:         "git:github.com/example-org/pi-ext@v1.0.0",
		},
	}
}

func TestVerifyCI_CleanAfterApplyingPackageDeclarations(t *testing.T) {
	project := setupCIProjectWith(t, declarePackagePlugins)

	var piDoc struct {
		Packages []string `json:"packages"`
	}
	readJSONInto(t, filepath.Join(project, ".pi", "settings.json"), &piDoc)
	want := "git:github.com/example-org/pi-ext@v1.0.0 npm:@example-org/agent-tools@1.2.3"
	if got := strings.Join(piDoc.Packages, " "); got != want {
		t.Errorf("pi packages = %q, want %q", got, want)
	}
	var ocDoc struct {
		Plugin []string       `json:"plugin"`
		MCP    map[string]any `json:"mcp"`
	}
	readJSONInto(t, filepath.Join(project, "opencode.json"), &ocDoc)
	if got := strings.Join(ocDoc.Plugin, " "); got != "@example-org/agent-tools@1.2.3" {
		t.Errorf("opencode plugin = %q, want only the npm package", got)
	}
	if ocDoc.MCP["docs"] == nil {
		t.Errorf("opencode.json must keep the MCP server written in the same apply, got %v", ocDoc.MCP)
	}

	out, err := runVerifyCIForTest(t)
	if err != nil {
		t.Fatalf("verify --ci should be clean right after apply, got %v\n%s", err, out)
	}
}

func TestVerifyCI_PackageOutsideAllowListIsAPolicyViolation(t *testing.T) {
	project := setupCIProjectWith(t, declarePackagePlugins)
	mcpAllowed := &domain.MCPServerAllowList{Names: []string{"docs"}, URLs: []string{"https://example.invalid/*"}}
	writeCIPolicy(t, project, domain.AllowLists{MCPServers: mcpAllowed, PluginSources: []string{"npm:@example-org/*"}})

	out, err := runVerifyCIForTest(t)
	requirePolicyViolation(t, err, out)
	if !strings.Contains(out, "github.com/example-org/pi-ext") {
		t.Errorf("output should name the package source, got:\n%s", out)
	}

	writeCIPolicy(t, project, domain.AllowLists{MCPServers: mcpAllowed, PluginSources: []string{"npm:@example-org/*", "github.com/example-org/*"}})
	if out, err := runVerifyCIForTest(t); err != nil {
		t.Errorf("allowed package sources must pass, got %v\n%s", err, out)
	}
}

func readJSONInto(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("apply should write %s: %v", path, err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}
