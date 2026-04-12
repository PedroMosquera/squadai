package agents

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/claude"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/codex"
	"github.com/PedroMosquera/agent-manager-pro/internal/adapters/opencode"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// ─── Interface compliance ───────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	inst := New(nil, "")
	if inst.ID() != domain.ComponentAgents {
		t.Errorf("ID() = %q, want %q", inst.ID(), domain.ComponentAgents)
	}
}

// ─── renderAgent ────────────────────────────────────────────────────────────

func TestRenderAgent_Full(t *testing.T) {
	def := domain.AgentDef{
		Description: "Code reviewer",
		Mode:        "subagent",
		Model:       "anthropic/claude-sonnet-4-5",
		Prompt:      "Review code for security issues.",
		Permission:  map[string]string{"edit": "deny", "bash": "deny"},
	}
	content := renderAgent("reviewer", def)
	if !strings.Contains(content, "description: Code reviewer") {
		t.Error("should contain description")
	}
	if !strings.Contains(content, "mode: subagent") {
		t.Error("should contain mode")
	}
	if !strings.Contains(content, "model: anthropic/claude-sonnet-4-5") {
		t.Error("should contain model")
	}
	if !strings.Contains(content, "permission:") {
		t.Error("should contain permission block")
	}
	if !strings.Contains(content, "  bash: deny") {
		t.Error("should contain bash permission")
	}
	if !strings.Contains(content, "Review code for security issues.") {
		t.Error("should contain prompt")
	}
	if !strings.HasPrefix(content, "---\n") {
		t.Error("should start with frontmatter")
	}
}

func TestRenderAgent_Minimal(t *testing.T) {
	def := domain.AgentDef{
		Description: "Helper",
	}
	content := renderAgent("helper", def)
	if !strings.Contains(content, "description: Helper") {
		t.Error("should contain description")
	}
	if strings.Contains(content, "mode:") {
		t.Error("should not contain mode when empty")
	}
	if strings.Contains(content, "model:") {
		t.Error("should not contain model when empty")
	}
}

// ─── Plan (OpenCode) ────────────────────────────────────────────────────────

func TestPlan_OpenCode_NewAgents_ReturnsCreate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testAgents(), project)

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
	expected := filepath.Join(project, ".opencode", "agents", "reviewer.md")
	if actions[0].TargetPath != expected {
		t.Errorf("TargetPath = %q, want %q", actions[0].TargetPath, expected)
	}
}

func TestPlan_OpenCode_UpToDate_ReturnsSkip(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	agentDefs := testAgents()
	inst := New(agentDefs, project)

	// Write the expected file.
	targetPath := filepath.Join(project, ".opencode", "agents", "reviewer.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatal(err)
	}
	content := renderAgent("reviewer", agentDefs["reviewer"])
	if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions[0].Action != domain.ActionSkip {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionSkip)
	}
}

func TestPlan_OpenCode_Outdated_ReturnsUpdate(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testAgents(), project)

	targetPath := filepath.Join(project, ".opencode", "agents", "reviewer.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("old content"), 0644); err != nil {
		t.Fatal(err)
	}

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions[0].Action != domain.ActionUpdate {
		t.Errorf("Action = %q, want %q", actions[0].Action, domain.ActionUpdate)
	}
}

// ─── Plan (unsupported adapters) ────────────────────────────────────────────

func TestPlan_Claude_ReturnsNil(t *testing.T) {
	project := t.TempDir()
	adapter := claude.New()
	inst := New(testAgents(), project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for claude, got %d", len(actions))
	}
}

func TestPlan_Codex_ReturnsNil(t *testing.T) {
	project := t.TempDir()
	adapter := codex.New()
	inst := New(testAgents(), project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions for codex, got %d", len(actions))
	}
}

func TestPlan_NoAgents_ReturnsEmpty(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(nil, project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_CreatesAgentFile(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testAgents(), project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	data, err := os.ReadFile(actions[0].TargetPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "description: Code reviewer") {
		t.Error("should contain agent description")
	}
	if !strings.Contains(string(data), "Review code carefully.") {
		t.Error("should contain agent prompt")
	}
}

func TestApply_SkipDoesNothing(t *testing.T) {
	inst := New(nil, "")
	err := inst.Apply(domain.PlannedAction{Action: domain.ActionSkip})
	if err != nil {
		t.Fatalf("Skip should succeed, got: %v", err)
	}
}

func TestApply_Idempotent(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testAgents(), project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(actions[0].TargetPath)

	actions2, _ := inst.Plan(adapter, t.TempDir(), project)
	if actions2[0].Action != domain.ActionSkip {
		t.Fatalf("second plan should be Skip, got %q", actions2[0].Action)
	}

	second, _ := os.ReadFile(actions[0].TargetPath)
	if string(first) != string(second) {
		t.Error("content should not change")
	}
}

func TestApply_Delete(t *testing.T) {
	project := t.TempDir()
	targetPath := filepath.Join(project, ".opencode", "agents", "old.md")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(targetPath, []byte("old agent"), 0644); err != nil {
		t.Fatal(err)
	}

	inst := New(nil, project)
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
	inst := New(testAgents(), project)

	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	results, err := inst.Verify(adapter, t.TempDir(), project)
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	for _, r := range results {
		if !r.Passed {
			t.Errorf("check %q failed: %s", r.Check, r.Message)
		}
	}
}

func TestVerify_FailsWhenFileMissing(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(testAgents(), project)

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	if len(results) == 0 {
		t.Fatal("expected verify results")
	}
	if results[0].Passed {
		t.Error("should fail when agent file is missing")
	}
}

func TestVerify_NoAgents_ReturnsNil(t *testing.T) {
	project := t.TempDir()
	adapter := opencode.New()
	inst := New(nil, project)

	results, _ := inst.Verify(adapter, t.TempDir(), project)
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

// ─── Prompt from file ───────────────────────────────────────────────────────

func TestNew_ResolvesPromptFile(t *testing.T) {
	project := t.TempDir()
	amDir := filepath.Join(project, ".agent-manager")
	if err := os.MkdirAll(amDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(amDir, "reviewer-prompt.md"), []byte("File-based prompt."), 0644); err != nil {
		t.Fatal(err)
	}

	agents := map[string]domain.AgentDef{
		"reviewer": {
			Description: "Reviewer",
			Mode:        "subagent",
			PromptFile:  "reviewer-prompt.md",
		},
	}
	inst := New(agents, project)

	// The prompt should be resolved from file.
	adapter := opencode.New()
	actions, _ := inst.Plan(adapter, t.TempDir(), project)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(actions[0].TargetPath)
	if !strings.Contains(string(data), "File-based prompt.") {
		t.Error("should contain file-based prompt content")
	}
}

// ─── Helpers ────────────────────────────────────────────────────────────────

func testAgents() map[string]domain.AgentDef {
	return map[string]domain.AgentDef{
		"reviewer": {
			Description: "Code reviewer",
			Mode:        "subagent",
			Model:       "anthropic/claude-sonnet-4-5",
			Prompt:      "Review code carefully.",
			Permission:  map[string]string{"edit": "deny"},
		},
	}
}
