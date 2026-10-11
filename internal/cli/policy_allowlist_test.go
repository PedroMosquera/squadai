package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/exitcode"
	"github.com/PedroMosquera/squadai/internal/pipeline"
	"github.com/PedroMosquera/squadai/internal/planner"
)

func writePolicyFile(t *testing.T, project string, pol *domain.PolicyConfig) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(project, config.ProjectConfigDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteJSON(config.PolicyConfigPath(project), pol); err != nil {
		t.Fatal(err)
	}
}

func writeProjectFile(t *testing.T, project string, proj *domain.ProjectConfig) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(project, config.ProjectConfigDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), proj); err != nil {
		t.Fatal(err)
	}
}

func requirePolicyViolation(t *testing.T, err error, out string) {
	t.Helper()
	var appErr *exitcode.AppError
	if !errors.As(err, &appErr) || appErr.Code != exitcode.Policy {
		t.Fatalf("want exit code %d (policy), got %v\n%s", exitcode.Policy, err, out)
	}
}

func allowListProject() *domain.ProjectConfig {
	return &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentClaudeCode): {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentAgents): {Enabled: true},
		},
		Agents: map[string]domain.AgentDef{
			"reviewer": {Description: "Reviews diffs", Mode: "subagent", Model: "claude-opus-4-7"},
		},
		MCP: map[string]domain.MCPServerDef{
			"docs": {Type: "remote", URL: "https://example.invalid/mcp", Enabled: true},
		},
		Hooks: domain.HooksConfig{
			"Stop": {{Hooks: []domain.HookEntry{{Type: "command", Command: "./scripts/notify.sh"}}}},
		},
	}
}

func TestValidatePolicy_RejectsModelOutsideAllowedModels(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	writeProjectFile(t, project, allowListProject())
	pol := domain.DefaultPolicyConfig()
	pol.Allowed = domain.AllowLists{Models: []string{"claude-sonnet-*"}}
	writePolicyFile(t, project, pol)

	var buf bytes.Buffer
	err := RunValidatePolicy(nil, &buf)
	requirePolicyViolation(t, err, buf.String())
	if !strings.Contains(buf.String(), "claude-opus-4-7") || !strings.Contains(buf.String(), "reviewer") {
		t.Errorf("output should name the agent and the disallowed model, got:\n%s", buf.String())
	}
}

func TestValidatePolicy_AbsentAllowListsChangeNothing(t *testing.T) {
	project := t.TempDir()
	t.Chdir(project)
	writeProjectFile(t, project, allowListProject())
	writePolicyFile(t, project, domain.DefaultPolicyConfig())

	var buf bytes.Buffer
	if err := RunValidatePolicy(nil, &buf); err != nil {
		t.Fatalf("a policy without 'allowed' must not restrict project config, got %v\n%s", err, buf.String())
	}
	data, err := os.ReadFile(config.PolicyConfigPath(project))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "allowed") {
		t.Errorf("a policy without allow-lists must not serialize an 'allowed' key:\n%s", data)
	}
}

func TestApply_AllowedModelsWriteClaudeAvailableModels(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	home := filepath.Join(root, "home")
	writeProjectFile(t, project, &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{string(domain.AgentClaudeCode): {
			Enabled:  true,
			Settings: map[string]interface{}{"spinnerTipsEnabled": false},
		}},
		Components: map[string]domain.ComponentConfig{string(domain.ComponentSettings): {Enabled: true}},
	})
	writePolicyFile(t, project, &domain.PolicyConfig{Version: 1, Mode: domain.ModeTeam,
		Allowed: domain.AllowLists{Models: []string{"claude-sonnet-5-*"}}})

	merged, err := loadAndMerge(home, project)
	if err != nil {
		t.Fatal(err)
	}
	p := planner.New()
	actions, err := p.Plan(merged, allAdapters(), home, project)
	if err != nil {
		t.Fatal(err)
	}
	report, err := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot, nil).Execute(actions)
	if err != nil || !report.Success {
		t.Fatalf("apply failed: %v", err)
	}

	var settings struct {
		AvailableModels []string `json:"availableModels"`
	}
	data, err := os.ReadFile(filepath.Join(project, ".claude", "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatal(err)
	}
	if len(settings.AvailableModels) != 1 || settings.AvailableModels[0] != "claude-sonnet-5" {
		t.Errorf("want availableModels [claude-sonnet-5] in project settings, got:\n%s", data)
	}
}

func writeCIPolicy(t *testing.T, project string, allowed domain.AllowLists) {
	t.Helper()
	writePolicyFile(t, project, &domain.PolicyConfig{Version: 1, Mode: domain.ModePersonal, Allowed: allowed})
}

func TestVerifyCI_DisallowedMCPServerIsAPolicyViolation(t *testing.T) {
	project := setupCIProject(t)
	writeCIPolicy(t, project, domain.AllowLists{MCPServers: &domain.MCPServerAllowList{Names: []string{"context7"}}})

	for _, args := range [][]string{nil, {"--json"}} {
		out, err := runVerifyCIForTest(t, args...)
		requirePolicyViolation(t, err, out)
		if !strings.Contains(out, `"docs"`) && !strings.Contains(out, `\"docs\"`) {
			t.Errorf("args %v: output should name the disallowed server, got:\n%s", args, out)
		}
	}
}

func TestVerifyCI_AllowedMCPServerPasses(t *testing.T) {
	project := setupCIProject(t)
	writeCIPolicy(t, project, domain.AllowLists{MCPServers: &domain.MCPServerAllowList{
		Names: []string{"docs"},
		URLs:  []string{"https://example.invalid/*"},
	}})

	out, err := runVerifyCIForTest(t)
	if err != nil {
		t.Fatalf("an allowed server must pass, got %v\n%s", err, out)
	}
}
