package hooks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// ─── ID / interface ──────────────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	if got := New(nil).ID(); got != domain.ComponentHooks {
		t.Errorf("ID() = %q, want %q", got, domain.ComponentHooks)
	}
}

// ─── Plan ────────────────────────────────────────────────────────────────────

func TestPlan_NonClaudeAdapterIsNoop(t *testing.T) {
	dir := t.TempDir()
	inst := New(domain.HooksConfig{
		"PreToolUse": {{Hooks: []domain.HookEntry{{Type: "command", Command: "echo"}}}},
	})
	actions, err := inst.Plan(opencode.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 0 {
		t.Errorf("expected zero actions for non-Claude adapter, got %d", len(actions))
	}
}

func TestPlan_EmptyHooksIsNoop(t *testing.T) {
	dir := t.TempDir()
	actions, err := New(nil).Plan(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 0 {
		t.Errorf("expected zero actions for empty hooks config, got %d", len(actions))
	}
}

func TestPlan_CreateWhenFileDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	inst := New(domain.HooksConfig{
		"PreToolUse": {{Hooks: []domain.HookEntry{{Type: "command", Command: "echo pre"}}}},
	})
	actions, err := inst.Plan(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Action != domain.ActionCreate {
		t.Errorf("action = %v, want Create", actions[0].Action)
	}
}

func TestPlan_UpdateWhenFileExistsButHooksMissing(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{"agent": "orchestrator"})

	inst := New(domain.HooksConfig{
		"PreToolUse": {{Hooks: []domain.HookEntry{{Type: "command", Command: "echo pre"}}}},
	})
	actions, err := inst.Plan(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Action != domain.ActionUpdate {
		t.Errorf("expected Update, got %+v", actions)
	}
}

func TestPlan_SkipWhenAlreadyInstalled(t *testing.T) {
	dir := t.TempDir()
	want := domain.HooksConfig{
		"PreToolUse": {{Hooks: []domain.HookEntry{{Type: "command", Command: "echo pre"}}}},
	}
	if _, err := claude.SetHooks(dir, want); err != nil {
		t.Fatal(err)
	}

	actions, err := New(want).Plan(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Action != domain.ActionSkip {
		t.Errorf("expected Skip when already installed, got %+v", actions)
	}
}

// ─── Apply ───────────────────────────────────────────────────────────────────

func TestApply_SkipIsNoop(t *testing.T) {
	inst := New(domain.HooksConfig{})
	if err := inst.Apply(domain.PlannedAction{Action: domain.ActionSkip}); err != nil {
		t.Errorf("Apply on Skip should be no-op, got %v", err)
	}
}

func TestApply_WritesHooksToSettings(t *testing.T) {
	dir := t.TempDir()
	want := domain.HooksConfig{
		"PreToolUse": {{Hooks: []domain.HookEntry{{Type: "command", Command: "echo pre"}}}},
	}
	inst := New(want)
	actions, _ := inst.Plan(claude.New(), "", dir)

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	installed, err := claude.HooksInstalled(dir, want)
	if err != nil {
		t.Fatal(err)
	}
	if !installed {
		t.Error("hooks should be installed after Apply")
	}
}

func TestApply_IsIdempotent(t *testing.T) {
	dir := t.TempDir()
	want := domain.HooksConfig{
		"PreToolUse": {{Hooks: []domain.HookEntry{{Type: "command", Command: "echo pre"}}}},
	}
	inst := New(want)
	actions, _ := inst.Plan(claude.New(), "", dir)

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}
	// Second apply — action would be Skip but we can also call Apply directly.
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	installed, _ := claude.HooksInstalled(dir, want)
	if !installed {
		t.Error("hooks should still be installed after double Apply")
	}
}

// ─── Verify ──────────────────────────────────────────────────────────────────

func TestVerify_NonClaudeAdapterReturnsNothing(t *testing.T) {
	dir := t.TempDir()
	results, err := New(domain.HooksConfig{
		"PreToolUse": {{Hooks: []domain.HookEntry{{Type: "command", Command: "echo"}}}},
	}).Verify(opencode.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results for non-Claude adapter, got %d", len(results))
	}
}

func TestVerify_EmptyHooksReturnsNothing(t *testing.T) {
	dir := t.TempDir()
	results, err := New(nil).Verify(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results for empty hooks config, got %d", len(results))
	}
}

func TestVerify_PassWhenInstalled(t *testing.T) {
	dir := t.TempDir()
	want := domain.HooksConfig{
		"PreToolUse": {{Hooks: []domain.HookEntry{{Type: "command", Command: "echo pre"}}}},
	}
	if _, err := claude.SetHooks(dir, want); err != nil {
		t.Fatal(err)
	}

	results, err := New(want).Verify(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].Passed {
		t.Errorf("expected single passing result, got %+v", results)
	}
}

func TestVerify_FailWhenMissing(t *testing.T) {
	dir := t.TempDir()
	want := domain.HooksConfig{
		"PreToolUse": {{Hooks: []domain.HookEntry{{Type: "command", Command: "echo pre"}}}},
	}
	results, err := New(want).Verify(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Passed {
		t.Errorf("expected single failing result, got %+v", results)
	}
}

// ─── RenderContent ───────────────────────────────────────────────────────────

func TestRenderContent_ShowsMergedHooks(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{"agent": "orchestrator"})

	want := domain.HooksConfig{
		"PreToolUse": {{Hooks: []domain.HookEntry{{Type: "command", Command: "echo pre"}}}},
	}
	inst := New(want)
	actions, _ := inst.Plan(claude.New(), "", dir)

	rendered, err := inst.RenderContent(actions[0])
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(rendered, &got); err != nil {
		t.Fatalf("rendered output is not valid JSON: %v", err)
	}
	if got["agent"] != "orchestrator" {
		t.Errorf("rendered output dropped sibling key: %v", got)
	}
	if _, hasHooks := got["hooks"]; !hasHooks {
		t.Error("rendered output missing hooks key")
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

func writeSettings(t *testing.T, projectDir string, doc map[string]any) {
	t.Helper()
	dir := filepath.Join(projectDir, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
}
