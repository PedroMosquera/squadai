package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/codex"
	"github.com/PedroMosquera/squadai/internal/domain"
)

const deprecatedGithubPackage = "@modelcontextprotocol/server-github"

func catalogGithubDef(t *testing.T) domain.MCPServerDef {
	t.Helper()
	for _, s := range domain.DefaultMCPCatalog() {
		if s.Name == "github" {
			return domain.MCPServerDef{Type: s.Type, URL: s.URL, Command: append([]string{s.Command}, s.Args...), Enabled: true}
		}
	}
	t.Fatal("github server not found in catalog")
	return domain.MCPServerDef{}
}

func TestGithubServer_ReapplyReplacesDeprecatedNpmDefinition(t *testing.T) {
	old := domain.MCPServerDef{Type: "local", Command: []string{"npx", "-y", deprecatedGithubPackage + "@2025.4.8"}, Enabled: true}
	assertReapplyReplacesOwnedServer(t, "github", old, catalogGithubDef(t), deprecatedGithubPackage, "ghcr.io/github/github-mcp-server:v2.0.2")
}

func TestGithubServer_RendersPerHarnessFormat(t *testing.T) {
	args := []any{"run", "-i", "--rm", "-e", "GITHUB_PERSONAL_ACCESS_TOKEN", "ghcr.io/github/github-mcp-server:v2.0.2"}
	split := map[string]any{"command": "docker", "args": args}
	want := map[domain.AgentID]map[string]any{
		domain.AgentClaudeCode:    split,
		domain.AgentCursor:        split,
		domain.AgentWindsurf:      split,
		domain.AgentVSCodeCopilot: split,
		domain.AgentPi:            split,
		domain.AgentOpenCode:      {"type": "local", "command": append([]any{"docker"}, args...)},
	}
	servers := map[string]domain.MCPServerDef{"github": catalogGithubDef(t)}

	for _, adapter := range ownershipAdapters() {
		t.Run(string(adapter.ID()), func(t *testing.T) {
			project := t.TempDir()
			planAndApply(t, New(servers), adapter, project)
			var got map[string]any
			if err := json.Unmarshal(rawServer(t, mcpTarget(adapter, project), adapter.MCPRootKey(), "github"), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want[adapter.ID()]) {
				t.Errorf("github entry:\n got  %v\n want %v", got, want[adapter.ID()])
			}
		})
	}

	t.Run(string(domain.AgentCodex), func(t *testing.T) {
		home := t.TempDir()
		inst := New(servers)
		actions, err := inst.Plan(codex.New(), home, t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range actions {
			if err := inst.Apply(a); err != nil {
				t.Fatal(err)
			}
		}
		data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
		if err != nil {
			t.Fatal(err)
		}
		block := "[mcp_servers.github]\n" +
			"command = \"docker\"\n" +
			"args = [\"run\", \"-i\", \"--rm\", \"-e\", \"GITHUB_PERSONAL_ACCESS_TOKEN\", \"ghcr.io/github/github-mcp-server:v2.0.2\"]\n"
		if !strings.Contains(string(data), block) {
			t.Errorf("codex config missing\n%s\ngot:\n%s", block, data)
		}
	})
}
