package verify

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/copilot"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/memory"
	"github.com/PedroMosquera/agent-manager-pro/internal/components/rules"
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

// ─── Rules verification ─────────────────────────────────────────────────────

func TestVerify_RulesPass_AfterApply(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	standards := "Always run go vet."

	// Simulate apply: write AGENTS.md with team-standards marker.
	targetPath := filepath.Join(project, "AGENTS.md")
	content := marker.InjectSection("", rules.SectionID, standards)
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"rules": {Enabled: true},
		},
		Rules: domain.RulesConfig{
			TeamStandards: standards,
		},
	}

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), project)
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

	// Should have 3 rules checks: file-exists, markers-present, content-current.
	rulesChecks := 0
	for _, r := range report.Results {
		if r.Check == "rules-file-exists" || r.Check == "rules-markers-present" || r.Check == "rules-content-current" {
			rulesChecks++
		}
	}
	if rulesChecks != 3 {
		t.Errorf("expected 3 rules checks, got %d", rulesChecks)
	}
}

func TestVerify_RulesFails_WhenFileMissing(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"rules": {Enabled: true},
		},
		Rules: domain.RulesConfig{
			TeamStandards: "Standards.",
		},
	}

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.AllPass {
		t.Error("should fail when rules file is missing")
	}
}

func TestVerify_RulesFails_WhenContentOutdated(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()

	// Write file with old content.
	targetPath := filepath.Join(project, "AGENTS.md")
	content := marker.InjectSection("", rules.SectionID, "Old standards.")
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"rules": {Enabled: true},
		},
		Rules: domain.RulesConfig{
			TeamStandards: "New standards.",
		},
	}

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.AllPass {
		t.Error("should fail when rules content is outdated")
	}
}

func TestVerify_RulesDisabled_Skipped(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"rules": {Enabled: false},
		},
		Rules: domain.RulesConfig{
			TeamStandards: "Should not be checked.",
		},
	}

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.AllPass {
		t.Error("disabled rules should not cause failures")
	}
	for _, r := range report.Results {
		if r.Check == "rules-file-exists" || r.Check == "rules-markers-present" || r.Check == "rules-content-current" {
			t.Errorf("unexpected rules check %q when rules disabled", r.Check)
		}
	}
}

func TestVerify_RulesEmptyContent_Skipped(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"rules": {Enabled: true},
		},
		Rules: domain.RulesConfig{}, // empty content
	}

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.AllPass {
		t.Error("empty rules content should not cause failures")
	}
}

func TestVerify_AllPass_MemoryAndRules(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	standards := "Always use gofmt."

	// Write AGENTS.md with both memory and rules sections.
	targetPath := filepath.Join(project, "AGENTS.md")
	doc := marker.InjectSection("", "memory", memory.TemplateForAgentID(domain.AgentOpenCode))
	doc = marker.InjectSection(doc, rules.SectionID, standards)
	if err := os.WriteFile(targetPath, []byte(doc), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{
			"memory": {Enabled: true},
			"rules":  {Enabled: true},
		},
		Rules: domain.RulesConfig{
			TeamStandards: standards,
		},
	}

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), project)
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

	// Should have both memory (3) and rules (3) checks = 6.
	if len(report.Results) != 6 {
		t.Errorf("expected 6 results (3 memory + 3 rules), got %d", len(report.Results))
	}
}
