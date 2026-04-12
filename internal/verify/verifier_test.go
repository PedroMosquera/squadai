package verify

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

func TestVerify_AllPass_AfterFullApply(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	// Create memory content at project-level AGENTS.md with OpenCode template.
	memoryPath := filepath.Join(project, "AGENTS.md")
	memContent := marker.InjectSection("", "memory", memory.TemplateForAgentID(domain.AgentOpenCode))
	if err := os.WriteFile(memoryPath, []byte(memContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create copilot instructions.
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

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.AllPass {
		for _, r := range report.Results {
			if !r.Passed {
				t.Errorf("check %q failed: %s", r.Check, r.Message)
			}
		}
	}
}

func TestVerify_FailsWhenMemoryMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
		},
	}

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.AllPass {
		t.Error("should fail when memory file is missing")
	}
}

func TestVerify_FailsWhenCopilotMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode:       domain.ModePersonal,
		Adapters:   map[string]domain.AdapterConfig{},
		Components: map[string]domain.ComponentConfig{},
		Copilot: domain.CopilotConfig{
			InstructionsTemplate: "standard",
		},
	}

	v := New()
	report, err := v.Verify(cfg, nil, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.AllPass {
		t.Error("should fail when copilot instructions missing")
	}
}

func TestVerify_IncludesPolicyViolations(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	cfg := &domain.MergedConfig{
		Mode:       domain.ModeTeam,
		Adapters:   map[string]domain.AdapterConfig{},
		Components: map[string]domain.ComponentConfig{},
		Violations: []string{"field X locked to Y, overriding Z"},
	}

	v := New()
	report, err := v.Verify(cfg, nil, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, r := range report.Results {
		if r.Check == "policy-override" {
			found = true
			if !r.Passed {
				t.Error("policy violations should be informational (passed=true)")
			}
		}
	}
	if !found {
		t.Error("expected policy-override check in results")
	}
}

func TestVerify_DisabledComponents_Skipped(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	cfg := &domain.MergedConfig{
		Mode: domain.ModePersonal,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: false},
		},
	}

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.AllPass {
		t.Error("disabled components should not cause failures")
	}
	if len(report.Results) != 0 {
		t.Errorf("expected 0 results for disabled components, got %d", len(report.Results))
	}
}

func TestVerify_EmptyConfig(t *testing.T) {
	v := New()
	report, err := v.Verify(&domain.MergedConfig{
		Adapters:   map[string]domain.AdapterConfig{},
		Components: map[string]domain.ComponentConfig{},
	}, nil, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.AllPass {
		t.Error("empty config should pass")
	}
}
