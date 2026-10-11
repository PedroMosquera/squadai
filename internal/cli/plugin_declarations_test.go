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
