package planner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/opencode"
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
