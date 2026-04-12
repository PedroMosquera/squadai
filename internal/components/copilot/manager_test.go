package copilot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/marker"
)

// standardCfg returns a CopilotConfig for the standard template.
func standardCfg() domain.CopilotConfig {
	return domain.CopilotConfig{InstructionsTemplate: "standard"}
}

// ─── Plan ───────────────────────────────────────────────────────────────────

func TestPlan_NewProject_ReturnsCreate(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	action, err := mgr.Plan(dir, standardCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Action != domain.ActionCreate {
		t.Errorf("Action = %q, want %q", action.Action, domain.ActionCreate)
	}
}

func TestPlan_ExistingFileNoSection_ReturnsUpdate(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	target := filepath.Join(dir, CopilotInstructionsPath)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("# Custom Instructions\n\nUser content here.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	action, err := mgr.Plan(dir, standardCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", action.Action, domain.ActionUpdate)
	}
}

func TestPlan_UpToDate_ReturnsSkip(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	target := filepath.Join(dir, CopilotInstructionsPath)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	content := marker.InjectSection("", SectionID, TemplateContent("standard"))
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	action, err := mgr.Plan(dir, standardCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Action != domain.ActionSkip {
		t.Errorf("Action = %q, want %q", action.Action, domain.ActionSkip)
	}
}

func TestPlan_OutdatedSection_ReturnsUpdate(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	target := filepath.Join(dir, CopilotInstructionsPath)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	content := marker.InjectSection("", SectionID, "old instructions")
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	action, err := mgr.Plan(dir, standardCfg())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if action.Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", action.Action, domain.ActionUpdate)
	}
}

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	err := mgr.Apply(dir, standardCfg())
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	target := filepath.Join(dir, CopilotInstructionsPath)
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if !marker.HasSection(string(data), SectionID) {
		t.Error("expected managed section markers in file")
	}
}

func TestApply_PreservesUserContent(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	target := filepath.Join(dir, CopilotInstructionsPath)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("# My Custom Section\n\nDo not touch this.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := mgr.Apply(dir, standardCfg()); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(target)
	s := string(data)
	if !strContains(s, "# My Custom Section") {
		t.Error("user content should be preserved")
	}
	if !strContains(s, "Do not touch this.") {
		t.Error("user content should be preserved")
	}
	if !marker.HasSection(s, SectionID) {
		t.Error("managed section should be injected")
	}
}

func TestApply_Idempotent(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	if err := mgr.Apply(dir, standardCfg()); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(dir, CopilotInstructionsPath)
	first, _ := os.ReadFile(target)

	if err := mgr.Apply(dir, standardCfg()); err != nil {
		t.Fatal(err)
	}
	second, _ := os.ReadFile(target)

	if string(first) != string(second) {
		t.Error("second apply should produce identical content")
	}
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestVerify_AllPass_AfterApply(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	if err := mgr.Apply(dir, standardCfg()); err != nil {
		t.Fatal(err)
	}

	results := mgr.Verify(dir, standardCfg())
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
}

func TestVerify_FailsWhenFileMissing(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	results := mgr.Verify(dir, standardCfg())
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("expected file-exists check to fail")
	}
}

func TestVerify_FailsWhenNoMarkers(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	target := filepath.Join(dir, CopilotInstructionsPath)
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("no managed content"), 0644); err != nil {
		t.Fatal(err)
	}

	results := mgr.Verify(dir, standardCfg())
	foundMarkerCheck := false
	for _, r := range results {
		if r.Check == "copilot-markers-present" {
			foundMarkerCheck = true
			if r.Passed {
				t.Error("markers check should fail")
			}
		}
	}
	if !foundMarkerCheck {
		t.Error("expected copilot-markers-present check")
	}
}

// ─── TemplateContent ────────────────────────────────────────────────────────

func TestTemplateContent_Standard(t *testing.T) {
	content := TemplateContent("standard")
	if content == "" {
		t.Error("standard template should not be empty")
	}
	if !strContains(content, "Team Standards") {
		t.Error("standard template should contain Team Standards heading")
	}
}

func TestTemplateContent_UnknownTreatedAsInline(t *testing.T) {
	content := TemplateContent("Some inline instructions")
	if content != "Some inline instructions" {
		t.Errorf("non-standard template ref should be treated as inline, got %q", content)
	}
}

// ─── Custom template modes ─────────────────────────────────────────────────

func TestApply_CustomInlineContent(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	customCfg := domain.CopilotConfig{
		InstructionsTemplate: "custom",
		CustomContent:        "## Our Custom Standards\n\nUse TypeScript strict mode.",
	}

	if err := mgr.Apply(dir, customCfg); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	target := filepath.Join(dir, CopilotInstructionsPath)
	data, _ := os.ReadFile(target)
	s := string(data)
	if !strContains(s, "Our Custom Standards") {
		t.Error("custom content should be in file")
	}
	if !strContains(s, "TypeScript strict mode") {
		t.Error("custom content should be in file")
	}
	if !marker.HasSection(s, SectionID) {
		t.Error("managed section markers should be present")
	}
}

func TestApply_FileTemplate(t *testing.T) {
	projectDir := t.TempDir()
	mgr := New()

	// Create template file.
	tmplDir := filepath.Join(projectDir, ".agent-manager", "templates")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}
	tmplContent := "## File-Based Instructions\n\nLoaded from a file."
	if err := os.WriteFile(filepath.Join(tmplDir, "copilot.md"), []byte(tmplContent), 0644); err != nil {
		t.Fatal(err)
	}

	fileCfg := domain.CopilotConfig{
		InstructionsTemplate: "file:templates/copilot.md",
	}

	if err := mgr.Apply(projectDir, fileCfg); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	target := filepath.Join(projectDir, CopilotInstructionsPath)
	data, _ := os.ReadFile(target)
	s := string(data)
	if !strContains(s, "File-Based Instructions") {
		t.Error("file-based content should be in output")
	}
	if !marker.HasSection(s, SectionID) {
		t.Error("managed section markers should be present")
	}
}

func TestVerify_CustomContent_Passes(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	customCfg := domain.CopilotConfig{
		InstructionsTemplate: "custom",
		CustomContent:        "## Custom Team Rules\n\nBe excellent.",
	}

	if err := mgr.Apply(dir, customCfg); err != nil {
		t.Fatal(err)
	}

	results := mgr.Verify(dir, customCfg)
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func strContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
