package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/config"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/managed"
	"github.com/PedroMosquera/squadai/internal/pipeline"
	"github.com/PedroMosquera/squadai/internal/planner"
)

func TestApply_ProjectSidecarHoldsNoPathOutsideTheProject(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)

	proj := &domain.ProjectConfig{
		Version: 1,
		Adapters: map[string]domain.AdapterConfig{
			string(domain.AgentClaudeCode): {Enabled: true},
			string(domain.AgentOpenCode):   {Enabled: true},
			string(domain.AgentPi):         {Enabled: true},
			string(domain.AgentCodex):      {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			string(domain.ComponentMemory):      {Enabled: true},
			string(domain.ComponentMCP):         {Enabled: true},
			string(domain.ComponentSettings):    {Enabled: true},
			string(domain.ComponentPermissions): {Enabled: true},
		},
		MCP: map[string]domain.MCPServerDef{
			"docs": {Type: "remote", URL: "https://example.invalid/mcp", Enabled: true},
		},
	}
	if err := os.MkdirAll(filepath.Join(project, config.ProjectConfigDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteJSON(config.ProjectConfigPath(project), proj); err != nil {
		t.Fatal(err)
	}
	merged, err := loadAndMerge(home, project)
	if err != nil {
		t.Fatal(err)
	}
	applyDefaultProfile(merged)
	p := planner.New()
	actions, err := p.Plan(merged, allAdapters(), home, project)
	if err != nil {
		t.Fatal(err)
	}
	homeCreates := 0
	for _, a := range actions {
		if a.Action == domain.ActionCreate && strings.HasPrefix(a.TargetPath, home) {
			homeCreates++
		}
	}
	if homeCreates == 0 {
		t.Fatal("precondition: the plan creates no file under HOME, so the test proves nothing")
	}
	report, err := pipeline.New(p.ComponentInstallers(), p.CopilotManager(), project, merged.Copilot, nil).Execute(actions)
	if err != nil || !report.Success {
		t.Fatalf("apply failed: %v", err)
	}

	data, err := os.ReadFile(managed.SidecarPath(project))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		ManagedFiles map[string]json.RawMessage `json:"managed_files"`
		CreatedFiles []string                   `json:"created_files"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	paths := append([]string{}, doc.CreatedFiles...)
	for p := range doc.ManagedFiles {
		paths = append(paths, p)
	}
	for _, p := range paths {
		if strings.HasPrefix(p, "..") || filepath.IsAbs(p) {
			t.Errorf("project sidecar records a path outside the project: %q", p)
		}
	}
	if strings.Contains(string(data), home) {
		t.Errorf("project sidecar leaks the home path:\n%s", data)
	}
}
