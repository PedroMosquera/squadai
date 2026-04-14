package planner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/windsurf"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/copilot"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/memory"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/marker"
)

func TestPlan_MemoryAndCopilot_BothEnabled(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Copilot: domain.CopilotConfig{
			InstructionsTemplate: "standard",
		},
	}

	adapters := []domain.Adapter{opencode.New()}
	p := New()

	actions, err := p.Plan(cfg, adapters, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(actions) < 2 {
		t.Fatalf("expected at least 2 actions (memory + copilot), got %d", len(actions))
	}

	hasMemory := false
	hasCopilot := false
	for _, a := range actions {
		if a.Component == domain.ComponentMemory {
			hasMemory = true
		}
		if a.ID == "copilot-instructions" {
			hasCopilot = true
		}
	}
	if !hasMemory {
		t.Error("expected memory action in plan")
	}
	if !hasCopilot {
		t.Error("expected copilot action in plan")
	}
}

func TestPlan_DisabledAdapter_SkipsMemory(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: false},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
	}

	adapters := []domain.Adapter{opencode.New()}
	p := New()

	actions, err := p.Plan(cfg, adapters, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, a := range actions {
		if a.Component == domain.ComponentMemory {
			t.Error("should not plan memory for disabled adapter")
		}
	}
}

func TestPlan_DisabledMemory_SkipsMemory(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: false},
		},
	}

	adapters := []domain.Adapter{opencode.New()}
	p := New()

	actions, err := p.Plan(cfg, adapters, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, a := range actions {
		if a.Component == domain.ComponentMemory {
			t.Error("should not plan memory when disabled")
		}
	}
}

func TestPlan_NoCopilotTemplate_SkipsCopilot(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Copilot: domain.CopilotConfig{}, // no template
	}

	adapters := []domain.Adapter{opencode.New()}
	p := New()

	actions, err := p.Plan(cfg, adapters, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, a := range actions {
		if a.ID == "copilot-instructions" {
			t.Error("should not plan copilot when template is empty")
		}
	}
}

func TestPlan_NoAdapters_OnlyCopilot(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode:       domain.ModeTeam,
		Adapters:   map[string]domain.AdapterConfig{},
		Components: map[string]domain.ComponentConfig{},
		Copilot: domain.CopilotConfig{
			InstructionsTemplate: "standard",
		},
	}

	p := New()

	actions, err := p.Plan(cfg, nil, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(actions) != 1 {
		t.Fatalf("expected 1 action (copilot only), got %d", len(actions))
	}
	if actions[0].ID != "copilot-instructions" {
		t.Errorf("expected copilot action, got %q", actions[0].ID)
	}
}

func TestPlan_UpToDate_AllSkip(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	// Pre-create memory content at project-level AGENTS.md with OpenCode template.
	promptPath := filepath.Join(project, "AGENTS.md")
	memContent := marker.InjectSection("", "memory", memory.TemplateForAgentID(domain.AgentOpenCode))
	if err := os.WriteFile(promptPath, []byte(memContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Pre-create copilot instructions with correct content.
	copilotPath := filepath.Join(project, copilot.CopilotInstructionsPath)
	if err := os.MkdirAll(filepath.Dir(copilotPath), 0755); err != nil {
		t.Fatal(err)
	}
	copilotContent := marker.InjectSection("", copilot.SectionID, copilot.TemplateContent("standard"))
	if err := os.WriteFile(copilotPath, []byte(copilotContent), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
		Copilot: domain.CopilotConfig{
			InstructionsTemplate: "standard",
		},
	}

	p := New()
	actions, err := p.Plan(cfg, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, a := range actions {
		if a.Action != domain.ActionSkip {
			t.Errorf("action %q should be skip, got %q", a.ID, a.Action)
		}
	}
}

// ─── Plugins component ───────────────────────────────────────────────────────

func TestPlan_Plugins_IncludedWhenEnabled(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"plugins": {Enabled: true},
		},
		Plugins: map[string]domain.PluginDef{
			"superpowers": {
				Enabled:         true,
				SupportedAgents: []string{"opencode"},
				InstallMethod:   "skill_files",
			},
		},
	}

	adapters := []domain.Adapter{opencode.New()}
	p := New()

	actions, err := p.Plan(cfg, adapters, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasPlugins := false
	for _, a := range actions {
		if a.Component == domain.ComponentPlugins {
			hasPlugins = true
		}
	}
	if !hasPlugins {
		t.Error("expected plugins action in plan when component is enabled")
	}
}

func TestPlan_Plugins_SkippedWhenDisabled(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"plugins": {Enabled: false},
		},
		Plugins: map[string]domain.PluginDef{
			"superpowers": {
				Enabled:         true,
				SupportedAgents: []string{"opencode"},
				InstallMethod:   "skill_files",
			},
		},
	}

	adapters := []domain.Adapter{opencode.New()}
	p := New()

	actions, err := p.Plan(cfg, adapters, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, a := range actions {
		if a.Component == domain.ComponentPlugins {
			t.Error("should not plan plugins when component is disabled")
		}
	}
}

func TestPlan_Plugins_SkippedWhenNoPluginsDefined(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"plugins": {Enabled: true},
		},
		Plugins: nil, // no plugins configured
	}

	adapters := []domain.Adapter{opencode.New()}
	p := New()

	actions, err := p.Plan(cfg, adapters, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, a := range actions {
		if a.Component == domain.ComponentPlugins {
			t.Error("should not plan plugins when no plugins are defined")
		}
	}
}

// ─── Workflows component ─────────────────────────────────────────────────────

func TestPlan_Workflows_IncludedWhenEnabled(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"windsurf": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"workflows": {Enabled: true},
		},
		Methodology: domain.MethodologyTDD,
	}

	adapters := []domain.Adapter{windsurf.New()}
	p := New()

	actions, err := p.Plan(cfg, adapters, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasWorkflows := false
	for _, a := range actions {
		if a.Component == domain.ComponentWorkflows {
			hasWorkflows = true
		}
	}
	if !hasWorkflows {
		t.Error("expected workflows action in plan when component is enabled and adapter supports workflows")
	}
}

func TestPlan_Workflows_SkippedWhenDisabled(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"windsurf": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"workflows": {Enabled: false},
		},
		Methodology: domain.MethodologyTDD,
	}

	adapters := []domain.Adapter{windsurf.New()}
	p := New()

	actions, err := p.Plan(cfg, adapters, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, a := range actions {
		if a.Component == domain.ComponentWorkflows {
			t.Error("should not plan workflows when component is disabled")
		}
	}
}

func TestPlan_Workflows_SkippedForAdapterWithoutWorkflowSupport(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	// OpenCode does NOT support workflows.
	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"workflows": {Enabled: true},
		},
		Methodology: domain.MethodologyTDD,
	}

	adapters := []domain.Adapter{opencode.New()}
	p := New()

	actions, err := p.Plan(cfg, adapters, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, a := range actions {
		if a.Component == domain.ComponentWorkflows {
			t.Error("opencode does not support workflows — should not get workflows actions")
		}
	}
}

func TestPlan_Workflows_SkippedWhenNoMethodology(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"windsurf": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"workflows": {Enabled: true},
		},
		Methodology: "", // no methodology
	}

	adapters := []domain.Adapter{windsurf.New()}
	p := New()

	actions, err := p.Plan(cfg, adapters, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, a := range actions {
		if a.Component == domain.ComponentWorkflows {
			t.Error("should not plan workflows when no methodology is set")
		}
	}
}

// ─── ComponentInstallers — nil-safety ────────────────────────────────────────

func TestComponentInstallers_BeforePlan_NoNilPanic(t *testing.T) {
	p := New()
	// Should not panic — pluginsInstaller and workflowsInstaller are nil before Plan().
	installers := p.ComponentInstallers()

	// memoryInstaller is always present.
	if _, ok := installers[domain.ComponentMemory]; !ok {
		t.Error("expected memory installer to always be present")
	}

	// plugins and workflows are absent before Plan() — that is correct.
	if _, ok := installers[domain.ComponentPlugins]; ok {
		t.Error("plugins installer should be absent before Plan() is called")
	}
	if _, ok := installers[domain.ComponentWorkflows]; ok {
		t.Error("workflows installer should be absent before Plan() is called")
	}
}

func TestComponentInstallers_AfterPlan_IncludesNewInstallers(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"windsurf": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"plugins":   {Enabled: true},
			"workflows": {Enabled: true},
		},
		Methodology: domain.MethodologySDD,
		Plugins: map[string]domain.PluginDef{
			"superpowers": {
				Enabled:         true,
				SupportedAgents: []string{"windsurf"},
				InstallMethod:   "skill_files",
			},
		},
	}

	p := New()
	if _, err := p.Plan(cfg, []domain.Adapter{windsurf.New()}, home, project); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	installers := p.ComponentInstallers()

	if _, ok := installers[domain.ComponentPlugins]; !ok {
		t.Error("expected plugins installer after Plan()")
	}
	if _, ok := installers[domain.ComponentWorkflows]; !ok {
		t.Error("expected workflows installer after Plan()")
	}
}
