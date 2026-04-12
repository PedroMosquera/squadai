package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/claude"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// ─── Interface compliance ───────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	inst := New(nil)
	if inst.ID() != domain.ComponentCommands {
		t.Errorf("ID() = %q, want %q", inst.ID(), domain.ComponentCommands)
	}
}

// ─── renderCommand ──────────────────────────────────────────────────────────

func TestRenderCommand_Full(t *testing.T) {
	def := domain.CommandDef{
		Description: "Run tests",
		Template:    "Run all tests and report failures.",
		Agent:       "reviewer",
		Model:       "anthropic/claude-sonnet-4-5",
	}
	content := renderCommand("test", def)
	if !strings.Contains(content, "description: Run tests") {
		t.Error("should contain description")
	}
	if !strings.Contains(content, "agent: reviewer") {
		t.Error("should contain agent")
	}
	if !strings.Contains(content, "model: anthropic/claude-sonnet-4-5") {
		t.Error("should contain model")
	}
	if !strings.Contains(content, "Run all tests and report failures.") {
		t.Error("should contain template")
	}
}

func TestRenderCommand_Minimal(t *testing.T) {
	def := domain.CommandDef{
		Description: "Simple",
	}
	content := renderCommand("simple", def)
	if !strings.Contains(content, "description: Simple") {
		t.Error("should contain description")
	}
	if strings.Contains(content, "agent:") {
		t.Error("should not contain agent when empty")
	}
}

// ─── Plan (OpenCode) ────────────────────────────────────────────────────────

func TestPlan_OpenCode_NewCommands_ReturnsCreate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testCommands())

	actions, err := inst.Plan(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionCreate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionCreate)
	}
	expected := filepath.Join(project, ".opencode", "commands", "test.md")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestPlan_OpenCode_UpToDate_ReturnsSkip(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	cmdDefs := testCommands()
	inst := New(cmdDefs)

	targetPath := filepath.Join(project, ".opencode", "commands", "test.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatal(err)
	}
	content := renderCommand("test", cmdDefs["test"])
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions[0].Action != domain.ActionSkip {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionSkip)
	}
}

func TestPlan_Claude_ReturnsNil(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	inst := New(testCommands())

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for claude, got %d", len(actions))
	}
}

func TestPlan_NoCommands_ReturnsEmpty(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(nil)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_CreatesCommandFile(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testCommands())

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, _ := os.ReadFile(actions[0].TargetPath)
	if !strings.Contains(string(data), "description: Run tests") {
		t.Error("should contain command description")
	}
}

func TestApply_SkipDoesNothing(t *testing.T) {
	inst := New(nil)
	err := inst.Apply(domain.PlannedAction{Action: domain.ActionSkip})
	if err != nil {
		t.Fatalf("Skip should succeed, got: %v", err)
	}
}

func TestApply_Idempotent(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testCommands())

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	actions2, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions2[0].Action != domain.ActionSkip {
		t.Fatalf("second plan should be Skip, got %q", actions2[0].Action)
	}
}

func TestApply_Delete(t *testing.T) {
	project := t.TempDir()
	targetPath := filepath.Join(project, ".opencode", "commands", "old.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	inst := New(nil)
	err := inst.Apply(domain.PlannedAction{
		Action:     domain.ActionDelete,
		TargetPath: targetPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(targetPath); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestVerify_AllPass_AfterApply(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testCommands())

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
}

func TestVerify_FailsWhenFileMissing(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testCommands())

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("should fail when command file is missing")
	}
}

func TestVerify_NoCommands_ReturnsNil(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(nil)

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func testCommands() map[string]domain.CommandDef {
	return map[string]domain.CommandDef{
		"test": {
			Description: "Run tests",
			Template:    "Run go test ./... and report results.",
		},
	}
}
