package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// mockAdapter implements domain.Adapter for testing.
type mockAdapter struct {
	id   domain.AgentID
	lane domain.AdapterLane
}

func (m *mockAdapter) ID() domain.AgentID                                      { return m.id }
func (m *mockAdapter) Lane() domain.AdapterLane                                { return m.lane }
func (m *mockAdapter) Detect(_ context.Context, homeDir string) (bool, bool, error) {
	return true, true, nil
}
func (m *mockAdapter) GlobalConfigDir(homeDir string) string       { return "" }
func (m *mockAdapter) SystemPromptFile(homeDir string) string      { return "" }
func (m *mockAdapter) SkillsDir(homeDir string) string             { return "" }
func (m *mockAdapter) SettingsPath(homeDir string) string          { return "" }
func (m *mockAdapter) SupportsComponent(c domain.ComponentID) bool { return false }
func (m *mockAdapter) ProjectConfigFile(projectDir string) string  { return "" }
func (m *mockAdapter) ProjectRulesFile(projectDir string) string   { return "" }
func (m *mockAdapter) ProjectAgentsDir(projectDir string) string   { return "" }
func (m *mockAdapter) ProjectSkillsDir(projectDir string) string   { return "" }
func (m *mockAdapter) ProjectCommandsDir(projectDir string) string { return "" }
func (m *mockAdapter) DelegationStrategy() domain.DelegationStrategy {
	return domain.DelegationSoloAgent
}
func (m *mockAdapter) SupportsSubAgents() bool            { return false }
func (m *mockAdapter) SubAgentsDir(homeDir string) string  { return "" }
func (m *mockAdapter) SupportsWorkflows() bool             { return false }
func (m *mockAdapter) WorkflowsDir(projectDir string) string { return "" }

// ─── Intro Screen ───────────────────────────────────────────────────────────

func TestIntroScreen_ShowsVersionAndMode(t *testing.T) {
	adapters := []domain.Adapter{
		&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam},
	}
	m := NewModel("1.0.0", domain.ModeTeam, adapters, "/tmp/home")

	view := m.View()
	if !strings.Contains(view, "agent-manager 1.0.0") {
		t.Error("intro should show version")
	}
	if !strings.Contains(view, "Mode: team") {
		t.Error("intro should show mode")
	}
	if !strings.Contains(view, "opencode (team)") {
		t.Error("intro should show detected adapter")
	}
}

func TestIntroScreen_ShowsMultipleAdapters(t *testing.T) {
	adapters := []domain.Adapter{
		&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam},
		&mockAdapter{id: domain.AgentClaudeCode, lane: domain.LanePersonal},
		&mockAdapter{id: domain.AgentVSCodeCopilot, lane: domain.LanePersonal},
	}
	m := NewModel("1.0.0", domain.ModeHybrid, adapters, "/tmp/home")

	view := m.View()
	if !strings.Contains(view, "opencode (team)") {
		t.Error("should show opencode")
	}
	if !strings.Contains(view, "claude-code (personal)") {
		t.Error("should show claude-code")
	}
	if !strings.Contains(view, "vscode-copilot (personal)") {
		t.Error("should show vscode-copilot")
	}
	if !strings.Contains(view, "Mode: hybrid") {
		t.Error("should show hybrid mode")
	}
}

func TestIntroScreen_NoAdapters(t *testing.T) {
	m := NewModel("1.0.0", domain.ModePersonal, nil, "/tmp/home")

	view := m.View()
	if !strings.Contains(view, "(none detected)") {
		t.Error("should show no adapters message")
	}
}

// ─── Screen Navigation ─────────────────────────────────────────────────────

func TestIntro_AnyKeyAdvancesToMenu(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")

	if m.screen != screenIntro {
		t.Fatal("should start on intro")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model := updated.(Model)
	if model.screen != screenMenu {
		t.Error("any key on intro should advance to menu")
	}
}

func TestMenu_ShowsAllItems(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu

	view := m.View()
	for _, item := range menuItems {
		if !strings.Contains(view, item.label) {
			t.Errorf("menu should contain %q", item.label)
		}
	}
}

func TestMenu_CursorNavigation(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu
	m.cursor = 0

	// Move down.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	model := updated.(Model)
	if model.cursor != 1 {
		t.Errorf("cursor = %d, want 1", model.cursor)
	}

	// Move down again.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.cursor != 2 {
		t.Errorf("cursor = %d, want 2", model.cursor)
	}

	// Move up.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(Model)
	if model.cursor != 1 {
		t.Errorf("cursor = %d, want 1", model.cursor)
	}
}

func TestMenu_CursorDoesNotGoNegative(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu
	m.cursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	model := updated.(Model)
	if model.cursor != 0 {
		t.Errorf("cursor = %d, want 0 (should not go negative)", model.cursor)
	}
}

func TestMenu_CursorDoesNotExceedMax(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu
	m.cursor = len(menuItems) - 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	model := updated.(Model)
	if model.cursor != len(menuItems)-1 {
		t.Errorf("cursor = %d, want %d (should not exceed max)", model.cursor, len(menuItems)-1)
	}
}

func TestMenu_QuitKeyQuits(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model := updated.(Model)
	if !model.quitting {
		t.Error("q key should quit")
	}
	if cmd == nil {
		t.Error("should return tea.Quit cmd")
	}
}

func TestMenu_SelectQuitItem(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu
	// Find "Quit" index.
	for i, item := range menuItems {
		if item.command == "quit" {
			m.cursor = i
			break
		}
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if !model.quitting {
		t.Error("selecting Quit should quit")
	}
	if cmd == nil {
		t.Error("should return tea.Quit cmd")
	}
}

func TestMenu_CtrlCQuits(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	model := updated.(Model)
	if !model.quitting {
		t.Error("ctrl+c should quit")
	}
	if cmd == nil {
		t.Error("should return tea.Quit cmd")
	}
}

func TestMenu_RestoreShowsMessage(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu
	// Find "Restore backup" index.
	for i, item := range menuItems {
		if item.command == "restore" {
			m.cursor = i
			break
		}
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenResult {
		t.Error("restore should go to result screen")
	}
	if !strings.Contains(model.output, "backup ID") {
		t.Error("restore output should mention backup ID requirement")
	}
}

// ─── Result Screen ──────────────────────────────────────────────────────────

func TestResult_AnyKeyReturnsToMenu(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenResult
	m.output = "some output"

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	model := updated.(Model)
	if model.screen != screenMenu {
		t.Error("any key on result should return to menu")
	}
	if model.output != "" {
		t.Error("output should be cleared")
	}
}

func TestResult_ShowsOutput(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenResult
	m.output = "All checks passed."

	view := m.View()
	if !strings.Contains(view, "All checks passed.") {
		t.Error("result should show output")
	}
}

func TestResult_ShowsError(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenResult
	m.err = fmt.Errorf("something failed")

	view := m.View()
	if !strings.Contains(view, "Error: something failed") {
		t.Error("result should show error")
	}
}

// ─── Command Result Message ─────────────────────────────────────────────────

func TestCommandResult_SetsOutput(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenRunning

	updated, _ := m.Update(commandResult{output: "plan output", err: nil})
	model := updated.(Model)
	if model.screen != screenResult {
		t.Error("command result should go to result screen")
	}
	if model.output != "plan output" {
		t.Errorf("output = %q, want %q", model.output, "plan output")
	}
}

// ─── View Quitting ──────────────────────────────────────────────────────────

func TestView_Quitting_ReturnsEmpty(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.quitting = true

	view := m.View()
	if view != "" {
		t.Errorf("quitting view = %q, want empty", view)
	}
}

// ─── Menu Cursor Rendering ──────────────────────────────────────────────────

func TestMenu_CursorIndicator(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu
	m.cursor = 0

	view := m.View()
	if !strings.Contains(view, "> Plan (dry-run)") {
		t.Error("first item should have cursor indicator")
	}
}
