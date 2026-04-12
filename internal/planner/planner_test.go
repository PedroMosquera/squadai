package planner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/copilot"
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

	// Pre-create memory prompt with correct content.
	promptPath := adapter.SystemPromptFile(home)
	os.MkdirAll(filepath.Dir(promptPath), 0755)
	memContent := marker.InjectSection("", "memory", memoryProtocolTemplate())
	os.WriteFile(promptPath, []byte(memContent), 0644)

	// Pre-create copilot instructions with correct content.
	copilotPath := filepath.Join(project, copilot.CopilotInstructionsPath)
	os.MkdirAll(filepath.Dir(copilotPath), 0755)
	copilotContent := marker.InjectSection("", copilot.SectionID, copilot.TemplateContent("standard"))
	os.WriteFile(copilotPath, []byte(copilotContent), 0644)

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

// memoryProtocolTemplate duplicates the expected content for test setup.
// This avoids importing the memory package's internal constant.
func memoryProtocolTemplate() string {
	// Must match memory.ProtocolTemplate() exactly.
	return `## Memory Protocol

You have access to persistent memory tools. Follow these rules:

### Save Triggers
Save context after any of these events:
- Important decisions or architecture choices
- Bug discoveries and their fixes
- New conventions or patterns established
- Configuration changes
- Dependency additions or removals

### Search Protocol
- At session start, search memory for relevant context
- Before making architectural decisions, check for prior decisions
- Use keyword search for specific topics

### Session Summary
At the end of each session, save a summary including:
- Goal: what was the objective
- Accomplished: what was completed
- Discoveries: what was learned
- Next Steps: what remains to be done`
}
