package copilot

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/marker"
)

// ─── Plan ───────────────────────────────────────────────────────────────────

func TestPlan_NewProject_ReturnsCreate(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	action, err := mgr.Plan(dir, "standard")
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
	os.MkdirAll(filepath.Dir(target), 0755)
	os.WriteFile(target, []byte("# Custom Instructions\n\nUser content here.\n"), 0644)

	action, err := mgr.Plan(dir, "standard")
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
	os.MkdirAll(filepath.Dir(target), 0755)
	content := marker.InjectSection("", SectionID, TemplateContent("standard"))
	os.WriteFile(target, []byte(content), 0644)

	action, err := mgr.Plan(dir, "standard")
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
	os.MkdirAll(filepath.Dir(target), 0755)
	content := marker.InjectSection("", SectionID, "old instructions")
	os.WriteFile(target, []byte(content), 0644)

	action, err := mgr.Plan(dir, "standard")
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

	err := mgr.Apply(dir, "standard")
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
	os.MkdirAll(filepath.Dir(target), 0755)
	os.WriteFile(target, []byte("# My Custom Section\n\nDo not touch this.\n"), 0644)

	mgr.Apply(dir, "standard")

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

	mgr.Apply(dir, "standard")

	target := filepath.Join(dir, CopilotInstructionsPath)
	first, _ := os.ReadFile(target)

	mgr.Apply(dir, "standard")
	second, _ := os.ReadFile(target)

	if string(first) != string(second) {
		t.Error("second apply should produce identical content")
	}
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestVerify_AllPass_AfterApply(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	mgr.Apply(dir, "standard")

	results := mgr.Verify(dir, "standard")
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
}

func TestVerify_FailsWhenFileMissing(t *testing.T) {
	dir := t.TempDir()
	mgr := New()

	results := mgr.Verify(dir, "standard")
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
	os.MkdirAll(filepath.Dir(target), 0755)
	os.WriteFile(target, []byte("no managed content"), 0644)

	results := mgr.Verify(dir, "standard")
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

func TestTemplateContent_UnknownFallsToStandard(t *testing.T) {
	content := TemplateContent("unknown-template")
	standard := TemplateContent("standard")
	if content != standard {
		t.Error("unknown template should fall back to standard")
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
