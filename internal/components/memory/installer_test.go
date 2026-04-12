package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
	"github.com/PedroMosquera/agent-manager-pro/internal/marker"
)

// ─── Interface compliance ───────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	inst := New()
	if inst.ID() != domain.ComponentMemory {
		t.Errorf("ID() = %q, want %q", inst.ID(), domain.ComponentMemory)
	}
}

// ─── Plan ───────────────────────────────────────────────────────────────────

func TestPlan_NewFile_ReturnsCreate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionCreate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionCreate)
	}
}

func TestPlan_ExistingFileNoSection_ReturnsUpdate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	// Create prompt file with existing content but no memory section.
	promptPath := adapter.SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(promptPath, []byte("# Existing Prompt\n\nSome content.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionUpdate)
	}
}

func TestPlan_UpToDate_ReturnsSkip(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	// Create file with correct memory section.
	promptPath := adapter.SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0755); err != nil {
		t.Fatal(err)
	}
	content := marker.InjectSection("", SectionID, ProtocolTemplate())
	if err := os.WriteFile(promptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionSkip {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionSkip)
	}
}

func TestPlan_OutdatedSection_ReturnsUpdate(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	promptPath := adapter.SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0755); err != nil {
		t.Fatal(err)
	}
	content := marker.InjectSection("", SectionID, "old protocol content")
	if err := os.WriteFile(promptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	actions, err := inst.Plan(adapter, home, project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionUpdate)
	}
}

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_CreatesFileWithMemorySection(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	actions, _ := inst.Plan(adapter, home, project)
	if len(actions) == 0 {
		t.Fatal("expected at least 1 action")
	}

	err := inst.Apply(actions[0])
	if err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	content, _ := os.ReadFile(actions[0].TargetPath)
	if !marker.HasSection(string(content), SectionID) {
		t.Error("expected memory marker section in file after apply")
	}
}

func TestApply_PreservesExistingContent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	promptPath := adapter.SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(promptPath, []byte("# My Custom Prompt\n\nDo not delete this.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, home, project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(promptPath)
	s := string(content)
	if !strContains(s, "# My Custom Prompt") {
		t.Error("existing content should be preserved")
	}
	if !strContains(s, "Do not delete this.") {
		t.Error("existing content should be preserved")
	}
	if !marker.HasSection(s, SectionID) {
		t.Error("memory section should be injected")
	}
}

func TestApply_SkipDoesNothing(t *testing.T) {
	inst := New()
	action := domain.PlannedAction{
		Action: domain.ActionSkip,
	}
	err := inst.Apply(action)
	if err != nil {
		t.Fatalf("Skip should succeed, got: %v", err)
	}
}

func TestApply_Idempotent(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	// First apply.
	actions, _ := inst.Plan(adapter, home, project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	first, _ := os.ReadFile(actions[0].TargetPath)

	// Second apply — should produce same content.
	actions2, _ := inst.Plan(adapter, home, project)
	if actions2[0].Action != domain.ActionSkip {
		t.Fatalf("second plan should be Skip, got %q", actions2[0].Action)
	}

	second, _ := os.ReadFile(actions[0].TargetPath)
	if string(first) != string(second) {
		t.Error("content should not change on second apply")
	}
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestVerify_AllPass_AfterApply(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	actions, _ := inst.Plan(adapter, home, project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	results, err := inst.Verify(adapter, home, project)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
}

func TestVerify_FailsWhenFileIsMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	results, err := inst.Verify(adapter, home, project)
	if err != nil {
		t.Fatalf("Verify error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("expected file-exists check to fail")
	}
}

func TestVerify_FailsWhenMarkersAreMissing(t *testing.T) {
	home := t.TempDir()
	project := t.TempDir()
	adapter := opencode.New()
	inst := New()

	promptPath := adapter.SystemPromptFile(home)
	if err := os.MkdirAll(filepath.Dir(promptPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(promptPath, []byte("no markers"), 0644); err != nil {
		t.Fatal(err)
	}

	results, _ := inst.Verify(adapter, home, project)

	foundMarkerCheck := false
	for _, r := range results {
		if r.Check == "memory-markers-present" {
			foundMarkerCheck = true
			if r.Passed {
				t.Error("markers-present should fail when no markers")
			}
		}
	}
	if !foundMarkerCheck {
		t.Error("expected memory-markers-present check in results")
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
