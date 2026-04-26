package agent_teams

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/adapters/claude"
	"github.com/PedroMosquera/squadai/internal/adapters/opencode"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// ─── ID / interface ─────────────────────────────────────────────────────────

func TestInstaller_ImplementsInterface(t *testing.T) {
	var _ domain.ComponentInstaller = (*Installer)(nil)
}

func TestInstaller_ID(t *testing.T) {
	if got := New(false).ID(); got != domain.ComponentAgentTeams {
		t.Errorf("ID() = %q, want %q", got, domain.ComponentAgentTeams)
	}
}

// ─── Plan ───────────────────────────────────────────────────────────────────

func TestPlan_NonClaudeAdapterIsNoop(t *testing.T) {
	dir := t.TempDir()
	inst := New(true)
	actions, err := inst.Plan(opencode.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 0 {
		t.Errorf("expected zero actions for non-Claude adapter, got %d", len(actions))
	}
}

func TestPlan_EnableNoSettingsFile_PlanCreate(t *testing.T) {
	dir := t.TempDir()
	inst := New(true)

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
	if !strings.Contains(actions[0].Description, "enable") {
		t.Errorf("description should mention enable: %q", actions[0].Description)
	}
}

func TestPlan_AlreadyEnabled_PlanSkip(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"env": map[string]any{claude.AgentTeamsEnvVar: "1"},
	})

	inst := New(true)
	actions, err := inst.Plan(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Action != domain.ActionSkip {
		t.Errorf("expected single Skip action, got %+v", actions)
	}
}

func TestPlan_EnableWithExistingSettingsFile_PlanUpdate(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"agent": "orchestrator",
	})

	inst := New(true)
	actions, err := inst.Plan(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Action != domain.ActionUpdate {
		t.Errorf("expected Update, got %+v", actions)
	}
}

func TestPlan_DisableWhenSet_PlanUpdate(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"env": map[string]any{claude.AgentTeamsEnvVar: "1"},
	})

	inst := New(false)
	actions, err := inst.Plan(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Action != domain.ActionUpdate {
		t.Errorf("expected Update for disable, got %+v", actions)
	}
	if !strings.Contains(actions[0].Description, "disable") {
		t.Errorf("description should mention disable: %q", actions[0].Description)
	}
}

func TestPlan_DisableWhenAlreadyMissing_PlanSkip(t *testing.T) {
	dir := t.TempDir()

	inst := New(false)
	actions, err := inst.Plan(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(actions) != 1 || actions[0].Action != domain.ActionSkip {
		t.Errorf("expected Skip when disabling not-set, got %+v", actions)
	}
}

// ─── Apply ──────────────────────────────────────────────────────────────────

func TestApply_EnableWritesEnvVar(t *testing.T) {
	dir := t.TempDir()
	inst := New(true)
	actions, _ := inst.Plan(claude.New(), "", dir)

	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	enabled, err := claude.AgentTeamsEnabled(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !enabled {
		t.Error("expected env var to be set after enable Apply")
	}
}

func TestApply_DisableRemovesEnvVar(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"env": map[string]any{claude.AgentTeamsEnvVar: "1"},
	})

	inst := New(false)
	actions, _ := inst.Plan(claude.New(), "", dir)
	if err := inst.Apply(actions[0]); err != nil {
		t.Fatal(err)
	}

	enabled, err := claude.AgentTeamsEnabled(dir)
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Error("expected env var removed after disable Apply")
	}
}

func TestApply_SkipIsNoop(t *testing.T) {
	inst := New(false)
	if err := inst.Apply(domain.PlannedAction{Action: domain.ActionSkip}); err != nil {
		t.Errorf("Apply on Skip should be no-op, got %v", err)
	}
}

// ─── Verify ─────────────────────────────────────────────────────────────────

func TestVerify_NonClaudeAdapterReturnsNothing(t *testing.T) {
	dir := t.TempDir()
	inst := New(true)
	results, err := inst.Verify(opencode.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 0 {
		t.Errorf("expected no results for non-Claude, got %d", len(results))
	}
}

func TestVerify_EnabledMatchesPasses(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"env": map[string]any{claude.AgentTeamsEnvVar: "1"},
	})

	results, err := New(true).Verify(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || !results[0].Passed {
		t.Errorf("expected single passing result, got %+v", results)
	}
}

func TestVerify_DesiredOnButMissing_Fails(t *testing.T) {
	dir := t.TempDir()
	results, err := New(true).Verify(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Passed {
		t.Errorf("expected failing result, got %+v", results)
	}
}

func TestVerify_DesiredOffButPresent_Fails(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"env": map[string]any{claude.AgentTeamsEnvVar: "1"},
	})

	results, err := New(false).Verify(claude.New(), "", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Passed {
		t.Errorf("expected failing result for stale env var, got %+v", results)
	}
}

// ─── RenderContent ──────────────────────────────────────────────────────────

func TestRenderContent_EnableShowsExpectedJSON(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"agent": "orchestrator",
	})

	inst := New(true)
	actions, _ := inst.Plan(claude.New(), "", dir)
	rendered, err := inst.RenderContent(actions[0])
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(rendered, &got); err != nil {
		t.Fatalf("rendered output is not valid JSON: %v", err)
	}
	envMap, _ := got["env"].(map[string]any)
	if envMap[claude.AgentTeamsEnvVar] != "1" {
		t.Errorf("rendered env var = %v, want \"1\"", envMap[claude.AgentTeamsEnvVar])
	}
	if got["agent"] != "orchestrator" {
		t.Errorf("rendered output dropped sibling key: %v", got)
	}
}

func TestRenderContent_DisableRemovesEnvKeyWhenLastEntry(t *testing.T) {
	dir := t.TempDir()
	writeSettings(t, dir, map[string]any{
		"env": map[string]any{claude.AgentTeamsEnvVar: "1"},
	})

	inst := New(false)
	actions, _ := inst.Plan(claude.New(), "", dir)
	rendered, err := inst.RenderContent(actions[0])
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(rendered, &got); err != nil {
		t.Fatalf("rendered output is not valid JSON: %v", err)
	}
	if _, exists := got["env"]; exists {
		t.Errorf("env key should be omitted when last entry removed, got %v", got["env"])
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
