package cli

import (
	"strings"
	"testing"

	"github.com/PedroMosquera/squadai/internal/domain"
)

func TestFormatPlannedActions_Verbose_PreservesPerActionLines(t *testing.T) {
	actions := []domain.PlannedAction{
		{Agent: "claude-code", Component: domain.ComponentSkills, Action: domain.ActionCreate, TargetPath: "/p/.claude/skills/code-review.md", Description: "skill code-review"},
		{Agent: "claude-code", Component: domain.ComponentAgents, Action: domain.ActionCreate, TargetPath: "/p/.claude/agents/orchestrator.md", Description: "agent orchestrator"},
	}
	got := formatPlannedActions(actions, true)
	if !strings.Contains(got, "/p/.claude/skills/code-review.md") {
		t.Errorf("verbose output missing target path; got:\n%s", got)
	}
	if !strings.Contains(got, "skill code-review") {
		t.Errorf("verbose output missing description; got:\n%s", got)
	}
	if strings.Contains(got, "create create") {
		t.Errorf("verbose output contains doubled-word; got:\n%s", got)
	}
}

func TestFormatPlannedActions_Grouped_GroupsByAgentAndAction(t *testing.T) {
	actions := []domain.PlannedAction{
		{Agent: "claude-code", Component: domain.ComponentSkills, Action: domain.ActionCreate},
		{Agent: "claude-code", Component: domain.ComponentSkills, Action: domain.ActionCreate},
		{Agent: "claude-code", Component: domain.ComponentAgents, Action: domain.ActionCreate},
		{Agent: "claude-code", Component: domain.ComponentSettings, Action: domain.ActionUpdate},
		{Agent: "opencode", Component: domain.ComponentSkills, Action: domain.ActionCreate},
		{Agent: "opencode", Component: domain.ComponentSkills, Action: domain.ActionCreate},
		{Agent: "opencode", Component: domain.ComponentSkills, Action: domain.ActionCreate},
	}

	got := formatPlannedActions(actions, false)

	cIdx := strings.Index(got, "claude-code (4)")
	oIdx := strings.Index(got, "opencode (3)")
	if cIdx < 0 {
		t.Errorf("missing claude-code header; got:\n%s", got)
	}
	if oIdx < 0 {
		t.Errorf("missing opencode header; got:\n%s", got)
	}
	if cIdx > oIdx {
		t.Errorf("claude-code should appear before opencode; got:\n%s", got)
	}

	if !strings.Contains(got, "create:") {
		t.Errorf("missing create: action line; got:\n%s", got)
	}
	if !strings.Contains(got, "update:") {
		t.Errorf("missing update: action line; got:\n%s", got)
	}

	if !strings.Contains(got, "skills(2)") {
		t.Errorf("missing skills(2) count under claude-code; got:\n%s", got)
	}
	if !strings.Contains(got, "agents(1)") {
		t.Errorf("missing agents(1) count; got:\n%s", got)
	}
	if !strings.Contains(got, "settings(1)") {
		t.Errorf("missing settings(1) count under update; got:\n%s", got)
	}

	if strings.Contains(got, "/p/") || strings.Contains(got, ".md") {
		t.Errorf("grouped output should not include target paths; got:\n%s", got)
	}
}

func TestFormatPlannedActions_Grouped_OrdersActionsCreateUpdateDeleteSkip(t *testing.T) {
	actions := []domain.PlannedAction{
		{Agent: "claude-code", Component: domain.ComponentMCP, Action: domain.ActionSkip},
		{Agent: "claude-code", Component: domain.ComponentSettings, Action: domain.ActionDelete},
		{Agent: "claude-code", Component: domain.ComponentSettings, Action: domain.ActionUpdate},
		{Agent: "claude-code", Component: domain.ComponentSkills, Action: domain.ActionCreate},
	}

	got := formatPlannedActions(actions, false)
	createIdx := strings.Index(got, "create:")
	updateIdx := strings.Index(got, "update:")
	deleteIdx := strings.Index(got, "delete:")
	skipIdx := strings.Index(got, "skip:")

	if !(createIdx < updateIdx && updateIdx < deleteIdx && deleteIdx < skipIdx) {
		t.Errorf("action ordering wrong; got:\n%s", got)
	}
}

func TestFormatPlannedActions_Grouped_HighestCountFirstWithinAction(t *testing.T) {
	actions := []domain.PlannedAction{
		{Agent: "windsurf", Component: domain.ComponentMemory, Action: domain.ActionCreate},
		{Agent: "windsurf", Component: domain.ComponentSkills, Action: domain.ActionCreate},
		{Agent: "windsurf", Component: domain.ComponentSkills, Action: domain.ActionCreate},
		{Agent: "windsurf", Component: domain.ComponentSkills, Action: domain.ActionCreate},
		{Agent: "windsurf", Component: domain.ComponentAgents, Action: domain.ActionCreate},
		{Agent: "windsurf", Component: domain.ComponentAgents, Action: domain.ActionCreate},
	}

	got := formatPlannedActions(actions, false)
	skillsIdx := strings.Index(got, "skills(3)")
	agentsIdx := strings.Index(got, "agents(2)")
	memoryIdx := strings.Index(got, "memory(1)")

	if !(skillsIdx < agentsIdx && agentsIdx < memoryIdx) {
		t.Errorf("expected components ordered by count desc; got:\n%s", got)
	}
}

func TestFormatPlannedActions_Empty_ReturnsEmptyString(t *testing.T) {
	if got := formatPlannedActions(nil, false); got != "" {
		t.Errorf("expected empty string for nil input; got %q", got)
	}
	if got := formatPlannedActions([]domain.PlannedAction{}, true); got != "" {
		t.Errorf("expected empty string for empty input; got %q", got)
	}
}
