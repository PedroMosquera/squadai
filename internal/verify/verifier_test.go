package verify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/adapters/windsurf"
	"github.com/PedroMosquera/squadai/internal/components/copilot"
	"github.com/PedroMosquera/squadai/internal/components/memory"
	"github.com/PedroMosquera/squadai/internal/components/rules"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/marker"
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
	for _, r := range report.Results {
		if r.Component != "health" && r.Component != "policy" {
			t.Errorf("unexpected non-health check %q when components disabled", r.Check)
		}
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

	// Should have both memory (3) and rules (3) checks + 1 health check = 7.
	if len(report.Results) != 7 {
		t.Errorf("expected 7 results (3 memory + 3 rules + 1 health), got %d", len(report.Results))
	}
}

// ─── Settings verification ──────────────────────────────────────────────────

func TestVerify_SettingsPass_AfterApply(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()

	// Write opencode.json with correct managed settings (including $schema).
	targetPath := filepath.Join(project, "opencode.json")
	writeVerifyJSON(t, targetPath, map[string]interface{}{
		"$schema": "https://opencode.ai/config.json",
		"model":   "anthropic/claude-sonnet-4-5",
	})

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true, Settings: map[string]interface{}{
				"model": "anthropic/claude-sonnet-4-5",
			}},
		},
		Components: map[string]domain.ComponentConfig{
			"settings": {Enabled: true},
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
	// Should have 2 settings checks: file-exists, keys-current.
	settingsChecks := 0
	for _, r := range report.Results {
		if r.Component == "settings" {
			settingsChecks++
		}
	}
	if settingsChecks != 2 {
		t.Errorf("expected 2 settings checks, got %d", settingsChecks)
	}
}

func TestVerify_SettingsFails_WhenFileMissing(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true, Settings: map[string]interface{}{
				"model": "anthropic/claude-sonnet-4-5",
			}},
		},
		Components: map[string]domain.ComponentConfig{
			"settings": {Enabled: true},
		},
	}

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.AllPass {
		t.Error("should fail when settings file is missing")
	}
}

func TestVerify_SettingsDisabled_Skipped(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true, Settings: map[string]interface{}{
				"model": "some-model",
			}},
		},
		Components: map[string]domain.ComponentConfig{
			"settings": {Enabled: false},
		},
	}

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.AllPass {
		t.Error("disabled settings should not cause failures")
	}
	for _, r := range report.Results {
		if r.Check == "settings-file-exists" || r.Check == "settings-keys-current" {
			t.Errorf("unexpected settings check %q when settings disabled", r.Check)
		}
	}
}

// writeVerifyJSON is a test helper to write a JSON file.
func writeVerifyJSON(t *testing.T, path string, data map[string]interface{}) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
}

// ─── Severity and component tagging ─────────────────────────────────────────

func TestVerify_ResultsHaveSeverity(t *testing.T) {
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

	for _, r := range report.Results {
		if r.Severity == "" {
			t.Errorf("result %q has empty severity", r.Check)
		}
		if r.Severity != domain.SeverityError && r.Severity != domain.SeverityWarning && r.Severity != domain.SeverityInfo {
			t.Errorf("result %q has unknown severity %q", r.Check, r.Severity)
		}
	}
}

func TestVerify_ResultsHaveComponent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	// Create memory content so we get passing results.
	memoryPath := filepath.Join(project, "AGENTS.md")
	memContent := marker.InjectSection("", "memory", memory.TemplateForAgentID(domain.AgentOpenCode))
	if err := os.WriteFile(memoryPath, []byte(memContent), 0644); err != nil {
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
	}

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, r := range report.Results {
		if r.Component == "" {
			t.Errorf("result %q has empty component", r.Check)
		}
	}

	// Memory results should be tagged with "memory".
	for _, r := range report.Results {
		if r.Check == "memory-file-exists" || r.Check == "memory-markers-present" || r.Check == "memory-content-current" {
			if r.Component != "memory" {
				t.Errorf("expected component 'memory' for check %q, got %q", r.Check, r.Component)
			}
		}
	}
}

func TestVerify_PassedResultsHaveInfoSeverity(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	// Create memory content.
	memoryPath := filepath.Join(project, "AGENTS.md")
	memContent := marker.InjectSection("", "memory", memory.TemplateForAgentID(domain.AgentOpenCode))
	if err := os.WriteFile(memoryPath, []byte(memContent), 0644); err != nil {
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
	}

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, r := range report.Results {
		if r.Passed && r.Severity != domain.SeverityInfo && r.Severity != domain.SeverityWarning {
			t.Errorf("passed result %q should be info or warning, got %q", r.Check, r.Severity)
		}
	}
}

func TestVerify_FailedResultsHaveErrorSeverity(t *testing.T) {
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

	for _, r := range report.Results {
		if !r.Passed && r.Severity != domain.SeverityError {
			t.Errorf("failed result %q should be error, got %q", r.Check, r.Severity)
		}
	}
}

func TestVerify_PolicyViolationsAreWarnings(t *testing.T) {
	cfg := &domain.MergedConfig{
		Mode:       domain.ModeTeam,
		Adapters:   map[string]domain.AdapterConfig{},
		Components: map[string]domain.ComponentConfig{},
		Violations: []string{"locked field overridden"},
	}

	v := New()
	report, err := v.Verify(cfg, nil, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, r := range report.Results {
		if r.Check == "policy-override" {
			if r.Severity != domain.SeverityWarning {
				t.Errorf("policy-override should be warning, got %q", r.Severity)
			}
			if r.Component != "policy" {
				t.Errorf("policy-override component should be 'policy', got %q", r.Component)
			}
		}
	}
}

// ─── Agent health checks ────────────────────────────────────────────────────

func TestVerify_HealthCheck_ConfiguredAndDetected(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{},
	}

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, r := range report.Results {
		if r.Check == "agent-opencode-detected" {
			found = true
			if !r.Passed {
				t.Error("opencode should be reported as detected")
			}
			if r.Component != "health" {
				t.Errorf("expected component 'health', got %q", r.Component)
			}
			if r.Severity != domain.SeverityInfo {
				t.Errorf("detected agent should be info severity, got %q", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected agent-opencode-detected health check")
	}
}

func TestVerify_HealthCheck_ConfiguredButNotDetected(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()

	// Configure claude-code but don't pass it as detected adapter.
	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode":    {Enabled: true},
			"claude-code": {Enabled: true},
		},
		Components: map[string]domain.ComponentConfig{},
	}

	// Only opencode is detected.
	adapter := opencode.New()

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, r := range report.Results {
		if r.Check == "agent-claude-code-detected" {
			found = true
			if r.Passed {
				t.Error("claude-code should fail — configured but not detected")
			}
			if r.Severity != domain.SeverityError {
				t.Errorf("configured-but-not-detected should be error, got %q", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected agent-claude-code-detected health check")
	}

	if report.AllPass {
		t.Error("should not pass when configured adapter is not detected")
	}
}

func TestVerify_HealthCheck_DetectedButNotConfigured(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()

	// Config does NOT include opencode.
	cfg := &domain.MergedConfig{
		Mode:       domain.ModeTeam,
		Adapters:   map[string]domain.AdapterConfig{},
		Components: map[string]domain.ComponentConfig{},
	}

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found := false
	for _, r := range report.Results {
		if r.Check == "agent-opencode-unconfigured" {
			found = true
			if !r.Passed {
				t.Error("unconfigured agent should pass (informational)")
			}
			if r.Severity != domain.SeverityWarning {
				t.Errorf("unconfigured agent should be warning, got %q", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected agent-opencode-unconfigured health check")
	}
}

func TestVerify_HealthCheck_DisabledAdapter_Skipped(t *testing.T) {
	cfg := &domain.MergedConfig{
		Mode: domain.ModeTeam,
		Adapters: map[string]domain.AdapterConfig{
			"opencode": {Enabled: false},
		},
		Components: map[string]domain.ComponentConfig{},
	}

	adapter := opencode.New()
	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Disabled adapter should not get a "detected" check (only "unconfigured" may appear).
	for _, r := range report.Results {
		if r.Check == "agent-opencode-detected" {
			t.Error("disabled adapter should not get a 'detected' health check")
		}
	}
}

func TestVerify_JSONOutput_HasSeverityAndComponent(t *testing.T) {
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

	// Verify JSON round-trip preserves severity and component.
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded domain.VerifyReport
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	for i, r := range decoded.Results {
		if r.Severity == "" {
			t.Errorf("result[%d] %q lost severity after JSON round-trip", i, r.Check)
		}
		if r.Component == "" {
			t.Errorf("result[%d] %q lost component after JSON round-trip", i, r.Check)
		}
	}
}

// ─── Plugins verification ────────────────────────────────────────────────────

func TestVerify_Plugins_FailsWhenSkillFileMissing(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()

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

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.AllPass {
		t.Error("should fail when plugin skill file is missing")
	}

	found := false
	for _, r := range report.Results {
		if r.Component == "plugins" {
			found = true
		}
	}
	if !found {
		t.Error("expected plugins component in verify results")
	}
}

func TestVerify_Plugins_PassesWhenSkillFilePresent(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()

	// Write the expected skill file content.
	skillsDir := filepath.Join(project, ".opencode", "skills", "superpowers")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "# Superpowers\n\nEnhanced coding capabilities with advanced code generation, refactoring, and analysis tools.\n"
	if err := os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

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

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, r := range report.Results {
		if r.Component == "plugins" && !r.Passed {
			t.Errorf("plugins check %q should pass when skill file is present: %s", r.Check, r.Message)
		}
	}
}

func TestVerify_Plugins_DisabledComponent_Skipped(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()

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

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.AllPass {
		t.Error("disabled plugins component should not cause failures")
	}
	for _, r := range report.Results {
		if r.Component == "plugins" {
			t.Errorf("should not have plugins results when component is disabled, got check %q", r.Check)
		}
	}
}

// ─── Workflows verification ──────────────────────────────────────────────────

func TestVerify_Workflows_FailsWhenWorkflowFileMissing(t *testing.T) {
	project := t.TempDir()
	adapter := windsurf.New()

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

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.AllPass {
		t.Error("should fail when workflow file is missing")
	}

	found := false
	for _, r := range report.Results {
		if r.Component == "workflows" {
			found = true
		}
	}
	if !found {
		t.Error("expected workflows component in verify results")
	}
}

func TestVerify_Workflows_DisabledComponent_Skipped(t *testing.T) {
	project := t.TempDir()
	adapter := windsurf.New()

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

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !report.AllPass {
		t.Error("disabled workflows component should not cause failures")
	}
	for _, r := range report.Results {
		if r.Component == "workflows" {
			t.Errorf("should not have workflows results when component is disabled, got check %q", r.Check)
		}
	}
}

func TestVerify_Workflows_SkippedForNonWorkflowAdapter(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New() // OpenCode does NOT support workflows.

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

	v := New()
	report, err := v.Verify(cfg, []domain.Adapter{adapter}, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// OpenCode returns nil from Verify (no workflow support), so no failures.
	for _, r := range report.Results {
		if r.Component == "workflows" {
			t.Errorf("opencode should not produce workflows results, got check %q", r.Check)
		}
	}
}
