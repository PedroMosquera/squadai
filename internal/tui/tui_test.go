package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// mockAdapter implements domain.Adapter for testing.
type mockAdapter struct {
	id   domain.AgentID
	lane domain.AdapterLane
}

func (m *mockAdapter) ID() domain.AgentID       { return m.id }
func (m *mockAdapter) Lane() domain.AdapterLane { return m.lane }
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
func (m *mockAdapter) SupportsSubAgents() bool                { return false }
func (m *mockAdapter) SubAgentsDir(homeDir string) string     { return "" }
func (m *mockAdapter) SupportsWorkflows() bool                { return false }
func (m *mockAdapter) WorkflowsDir(projectDir string) string  { return "" }
func (m *mockAdapter) MCPRootKey() string                     { return "mcpServers" }
func (m *mockAdapter) MCPURLKey() string                      { return "url" }
func (m *mockAdapter) MCPConfigPath(projectDir string) string { return "" }
func (m *mockAdapter) RulesFrontmatter() string               { return "" }

// ─── Intro Screen ───────────────────────────────────────────────────────────

func TestIntroScreen_ShowsVersionAndMode(t *testing.T) {
	adapters := []domain.Adapter{
		&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam},
	}
	m := NewModel("1.0.0", domain.ModeTeam, adapters, "/tmp/home")

	view := m.View()
	if !strings.Contains(view, "SquadAI v1.0.0") {
		t.Error("header should show version")
	}
	if !strings.Contains(view, "One config. Every AI agent. Zero drift.") {
		t.Error("header should show product tagline")
	}
	// Display name format: "  OpenCode"
	if !strings.Contains(view, "OpenCode") {
		t.Error("intro should show detected adapter display name")
	}
}

func TestIntroScreen_ShowsMultipleAdapters(t *testing.T) {
	adapters := []domain.Adapter{
		&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam},
		&mockAdapter{id: domain.AgentClaudeCode, lane: domain.LanePersonal},
		&mockAdapter{id: domain.AgentVSCodeCopilot, lane: domain.LanePersonal},
	}
	m := NewModel("1.0.0", domain.ModePersonal, adapters, "/tmp/home")

	view := m.View()
	// Display names instead of raw IDs
	if !strings.Contains(view, "OpenCode") {
		t.Error("should show OpenCode")
	}
	if !strings.Contains(view, "Claude Code") {
		t.Error("should show Claude Code")
	}
	if !strings.Contains(view, "VS Code Copilot") {
		t.Error("should show VS Code Copilot")
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
	if !strings.Contains(view, "> Init / Setup") {
		t.Error("first item should have cursor indicator")
	}
}

// ─── Menu new items ──────────────────────────────────────────────────────────

func TestMenu_ShowsInitSetup(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu

	view := m.View()
	if !strings.Contains(view, "Init / Setup") {
		t.Error("menu should contain 'Init / Setup'")
	}
}

func TestMenu_ShowsTeamStatus(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu

	view := m.View()
	if !strings.Contains(view, "Team Status") {
		t.Error("menu should contain 'Team Status'")
	}
}

// ─── Init Methodology Screen ─────────────────────────────────────────────────

func TestInitMethodology_Navigation(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMethodology
	m.initCursor = 0

	// Move down.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	model := updated.(Model)
	if model.initCursor != 1 {
		t.Errorf("initCursor = %d, want 1 after down", model.initCursor)
	}

	// Move up.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(Model)
	if model.initCursor != 0 {
		t.Errorf("initCursor = %d, want 0 after up", model.initCursor)
	}
}

func TestInitMethodology_SelectTDD(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMethodology
	m.initCursor = 0 // TDD is first

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.methodology != domain.MethodologyTDD {
		t.Errorf("methodology = %q, want %q", model.methodology, domain.MethodologyTDD)
	}
	if model.screen != screenInitModelTier {
		t.Errorf("screen = %d, want screenInitModelTier (%d)", model.screen, screenInitModelTier)
	}
}

func TestInitMethodology_SelectSDD(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMethodology
	m.initCursor = 1 // SDD is second

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.methodology != domain.MethodologySDD {
		t.Errorf("methodology = %q, want %q", model.methodology, domain.MethodologySDD)
	}
	if model.screen != screenInitModelTier {
		t.Errorf("screen = %d, want screenInitModelTier (%d)", model.screen, screenInitModelTier)
	}
}

func TestInitMethodology_SelectConventional(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMethodology
	m.initCursor = 2 // Conventional is third

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.methodology != domain.MethodologyConventional {
		t.Errorf("methodology = %q, want %q", model.methodology, domain.MethodologyConventional)
	}
	if model.screen != screenInitModelTier {
		t.Errorf("screen = %d, want screenInitModelTier (%d)", model.screen, screenInitModelTier)
	}
}

func TestInitMethodology_EscReturnsToAdapters(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMethodology

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)

	if model.screen != screenInitAdapters {
		t.Errorf("screen = %d, want screenInitAdapters (%d) after esc", model.screen, screenInitAdapters)
	}
}

// ─── Team Status Screen ───────────────────────────────────────────────────────

func TestTeamStatus_ShowsNoMethodology(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenTeamStatus
	m.methodology = "" // no methodology set

	view := m.View()
	if !strings.Contains(view, "No methodology") {
		t.Errorf("team status with no methodology should show 'No methodology', got:\n%s", view)
	}
}

func TestTeamStatus_ShowsTDDTeam(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenTeamStatus
	m.methodology = domain.MethodologyTDD

	view := m.View()
	if !strings.Contains(view, "orchestrator") {
		t.Errorf("TDD team status should show 'orchestrator', got:\n%s", view)
	}
}

func TestTeamStatus_EscReturnsToMenu(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenTeamStatus

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)

	if model.screen != screenMenu {
		t.Errorf("screen = %d, want screenMenu (%d) after esc", model.screen, screenMenu)
	}
}

// ─── Init MCP Screen ──────────────────────────────────────────────────────────

func TestInitMethodology_SelectGoesToModelTier(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMethodology
	m.initCursor = 0 // TDD

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitModelTier {
		t.Errorf("screen = %d, want screenInitModelTier (%d) after methodology select", model.screen, screenInitModelTier)
	}
}

func TestInitMCP_ToggleContext7(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMCP
	m.mcpSelections = map[string]bool{"context7": true}
	m.initCursor = 0

	// Space toggles context7 off.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model := updated.(Model)

	if model.mcpSelections["context7"] {
		t.Error("context7 should be toggled off after space")
	}
}

func TestInitMCP_EnterGoesToPlugins_WhenCustom(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMCP
	m.setupPreset = domain.PresetCustom
	m.mcpSelections = map[string]bool{"context7": true}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitPlugins {
		t.Errorf("screen = %d, want screenInitPlugins (%d) after enter (custom)", model.screen, screenInitPlugins)
	}
}

func TestInitMCP_EnterGoesToInstallSummary_WhenNonCustom(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMCP
	m.setupPreset = domain.PresetFullSquad
	m.mcpSelections = map[string]bool{"context7": true}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitInstallSummary {
		t.Errorf("screen = %d, want screenInitInstallSummary (%d) after enter (non-custom)", model.screen, screenInitInstallSummary)
	}
}

func TestInitMCP_EscGoesToModelTier_WhenCustom(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMCP
	m.setupPreset = domain.PresetCustom
	m.mcpSelections = map[string]bool{}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)

	if model.screen != screenInitModelTier {
		t.Errorf("screen = %d, want screenInitModelTier (%d) after esc (custom)", model.screen, screenInitModelTier)
	}
}

func TestInitMCP_EscGoesToAdapters_WhenNonCustom(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMCP
	m.setupPreset = domain.PresetFullSquad
	m.mcpSelections = map[string]bool{}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)

	if model.screen != screenInitAdapters {
		t.Errorf("screen = %d, want screenInitAdapters (%d) after esc (non-custom)", model.screen, screenInitAdapters)
	}
}

// ─── Init Plugins Screen ──────────────────────────────────────────────────────

func TestInitPlugins_EnterGoesToInstallSummary(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitPlugins
	m.pluginSelections = make(map[string]bool)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitInstallSummary {
		t.Errorf("screen = %d, want screenInitInstallSummary (%d) after enter", model.screen, screenInitInstallSummary)
	}
}

func TestInitPlugins_EscGoesToMCP(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitPlugins
	m.pluginSelections = make(map[string]bool)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)

	if model.screen != screenInitMCP {
		t.Errorf("screen = %d, want screenInitMCP (%d) after esc", model.screen, screenInitMCP)
	}
}

// ─── Style Tests ──────────────────────────────────────────────────────────────

func TestStyles_AllDefined(t *testing.T) {
	// Verify all 9 style variables can render without panicking.
	// In no-TTY test environments lipgloss passes text through unchanged,
	// so we just check each style renders the input text.
	inputs := []struct {
		name  string
		style lipgloss.Style
	}{
		{"panelStyle", panelStyle},
		{"headingStyle", headingStyle},
		{"activeStyle", activeStyle},
		{"mutedStyle", mutedStyle},
		{"badgeActiveStyle", badgeActiveStyle},
		{"badgeDisabledStyle", badgeDisabledStyle},
		{"methodologyBadgeStyle", methodologyBadgeStyle},
		{"errorStyle", errorStyle},
		{"successStyle", successStyle},
	}

	for _, s := range inputs {
		out := s.style.Render("test")
		if !strings.Contains(out, "test") {
			t.Errorf("style %s: Render(\"test\") = %q, expected it to contain \"test\"", s.name, out)
		}
	}
}

// ─── Intro Screen New Tests ───────────────────────────────────────────────────

func TestIntroScreen_ShowsDelegationStrategy(t *testing.T) {
	adapters := []domain.Adapter{
		&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam},
	}
	m := NewModel("1.0.0", domain.ModeTeam, adapters, "/tmp/home")

	view := m.View()
	// Delegation strategy is no longer displayed on the intro screen.
	// Verify the adapter display name is shown instead.
	if !strings.Contains(view, "OpenCode") {
		t.Errorf("intro should show adapter display name 'OpenCode', got:\n%s", view)
	}
}

// ─── Menu Tests ───────────────────────────────────────────────────────────────

func TestMenu_AllItemsPresent(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu

	view := m.View()
	expectedLabels := []string{
		"Init / Setup",
		"Plan (dry-run)",
		"Apply",
		"Team Status",
		"Verify",
		"Restore Backup",
		"Quit",
	}
	for _, label := range expectedLabels {
		if !strings.Contains(view, label) {
			t.Errorf("menu should contain %q, got:\n%s", label, view)
		}
	}
}

// ─── Methodology Screen Tests ─────────────────────────────────────────────────

func TestInitMethodology_ShowsTeamRoles(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMethodology
	m.initCursor = 0 // TDD selected

	view := m.View()
	// TDD team has: orchestrator, brainstormer, planner, implementer, reviewer, debugger
	if !strings.Contains(view, "brainstormer") {
		t.Errorf("methodology screen should show TDD team roles including 'brainstormer', got:\n%s", view)
	}
}

// ─── MCP Screen Tests ──────────────────────────────────────────────────────────

func TestInitMCP_ShowsCommandHint(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMCP
	m.mcpSelections = catalogPreCheckedSelections()

	view := m.View()
	// The catalog-driven view now shows descriptions instead of commands.
	// Verify that context7's description is shown.
	if !strings.Contains(view, "documentation lookup") {
		t.Errorf("MCP screen should show context7 description, got:\n%s", view)
	}
}

// ─── Plugins Screen Tests ─────────────────────────────────────────────────────

func TestInitPlugins_ShowsSupportedAgents(t *testing.T) {
	adapters := []domain.Adapter{
		&mockAdapter{id: domain.AgentClaudeCode, lane: domain.LanePersonal},
	}
	m := NewModel("1.0.0", domain.ModeTeam, adapters, "/tmp/home")
	m.screen = screenInitPlugins
	m.methodology = domain.MethodologySDD
	m.pluginSelections = make(map[string]bool)

	view := m.View()
	if !strings.Contains(view, "Supports:") {
		t.Errorf("plugins screen should show 'Supports:' for agents, got:\n%s", view)
	}
}

func TestInitPlugins_TDDBlockMessage(t *testing.T) {
	adapters := []domain.Adapter{
		&mockAdapter{id: domain.AgentClaudeCode, lane: domain.LanePersonal},
	}
	m := NewModel("1.0.0", domain.ModeTeam, adapters, "/tmp/home")
	m.screen = screenInitPlugins
	m.methodology = domain.MethodologyTDD
	m.pluginSelections = make(map[string]bool)

	view := m.View()
	// With TDD, superpowers is blocked — should show warning
	if !strings.Contains(view, "Superpowers") {
		t.Errorf("plugins screen with TDD should show Superpowers blocking message, got:\n%s", view)
	}
	if !strings.Contains(view, "TDD") {
		t.Errorf("plugins screen with TDD should mention TDD, got:\n%s", view)
	}
}

// ─── Team Status New Tests ─────────────────────────────────────────────────────

func TestTeamStatus_ShowsMCPSection(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenTeamStatus
	m.methodology = domain.MethodologyTDD
	m.mcpSelections = map[string]bool{"context7": true}

	view := m.View()
	if !strings.Contains(view, "MCP Servers") {
		t.Errorf("team status should show 'MCP Servers' heading, got:\n%s", view)
	}
}

func TestTeamStatus_ShowsPluginsSection(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenTeamStatus
	m.methodology = domain.MethodologyTDD

	view := m.View()
	if !strings.Contains(view, "Plugins") {
		t.Errorf("team status should show 'Plugins' heading, got:\n%s", view)
	}
}

func TestTeamStatus_MCPActiveWhenSelected(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenTeamStatus
	m.methodology = domain.MethodologyTDD
	m.mcpSelections = map[string]bool{"context7": true}

	view := m.View()
	if !strings.Contains(view, "context7") {
		t.Errorf("team status with context7 selected should show 'context7', got:\n%s", view)
	}
}

func TestTeamStatus_NotConfiguredWhenNil(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenTeamStatus
	m.methodology = domain.MethodologyTDD
	m.mcpSelections = nil // nil selections

	view := m.View()
	if !strings.Contains(view, "not configured") {
		t.Errorf("team status with nil mcpSelections should show 'not configured', got:\n%s", view)
	}
}

// ─── Result Screen New Tests ──────────────────────────────────────────────────

func TestViewResult_ShowsErrorMessage(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenResult
	m.err = fmt.Errorf("deployment failed: connection refused")

	view := m.View()
	if !strings.Contains(view, "deployment failed") {
		t.Errorf("result screen should show error message, got:\n%s", view)
	}
	if !strings.Contains(view, "Error:") {
		t.Errorf("result screen should show 'Error:' prefix, got:\n%s", view)
	}
}

func TestViewResult_ShowsSuccessMessage(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenResult
	m.output = "All 5 components applied successfully."
	m.err = nil

	view := m.View()
	if !strings.Contains(view, "All 5 components applied successfully.") {
		t.Errorf("result screen should show output, got:\n%s", view)
	}
}

// ─── Wizard Flow Tests ────────────────────────────────────────────────────────

func TestInitWizard_MCPPreSelectedContext7(t *testing.T) {
	// Context7 is pre-selected when entering screenInitMCP.
	// For Custom preset: this happens at screenInitModelTier enter.
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitModelTier
	m.setupPreset = domain.PresetCustom
	m.initCursor = 0 // Balanced

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitMCP {
		t.Errorf("after model tier select: screen = %d, want screenInitMCP (%d)", model.screen, screenInitMCP)
	}
	if model.mcpSelections == nil {
		t.Fatal("mcpSelections should be initialized after model tier selection")
	}
	if !model.mcpSelections["context7"] {
		t.Error("context7 should be pre-selected after model tier selection")
	}
}

func TestSortedKeys_DeterministicOrder(t *testing.T) {
	m := map[string]bool{
		"zebra":   true,
		"alpha":   false,
		"mango":   true,
		"banana":  false,
		"context": true,
	}

	keys := sortedKeys(m)
	expected := []string{"alpha", "banana", "context", "mango", "zebra"}

	if len(keys) != len(expected) {
		t.Fatalf("sortedKeys: len = %d, want %d", len(keys), len(expected))
	}
	for i, k := range keys {
		if k != expected[i] {
			t.Errorf("sortedKeys[%d] = %q, want %q", i, k, expected[i])
		}
	}
}

// ─── Skill Browser: Menu Access ───────────────────────────────────────────────

func TestMenu_ShowsBrowseSkills(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu

	view := m.View()
	if !strings.Contains(view, "Browse Skills") {
		t.Errorf("menu should contain 'Browse Skills', got:\n%s", view)
	}
}

func TestMenu_SelectBrowseSkills_GoesToSkillBrowser(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu
	// Navigate cursor to "Browse Skills".
	for i, item := range menuItems {
		if item.command == "skills" {
			m.cursor = i
			break
		}
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenSkillBrowser {
		t.Errorf("selecting 'Browse Skills' should go to screenSkillBrowser (%d), got screen %d",
			screenSkillBrowser, model.screen)
	}
}

// ─── Skill Browser: Rendering ─────────────────────────────────────────────────

func TestSkillBrowser_ShowsTitle(t *testing.T) {
	m := newSkillBrowserModel(t)

	view := m.View()
	if !strings.Contains(view, "Community Skills") {
		t.Errorf("skill browser should show 'Community Skills' title, got:\n%s", view)
	}
	if !strings.Contains(view, "skills.sh") {
		t.Errorf("skill browser should mention 'skills.sh', got:\n%s", view)
	}
}

func TestSkillBrowser_ShowsCategories(t *testing.T) {
	m := newSkillBrowserModel(t)

	view := m.View()
	// Catalog has categories; at least one should be visible.
	if len(m.skillCat.Categories) == 0 {
		t.Fatal("catalog should have at least one category")
	}
	firstCat := m.skillCat.Categories[0]
	if !strings.Contains(view, firstCat.Name) {
		t.Errorf("skill browser should show category %q, got:\n%s", firstCat.Name, view)
	}
}

func TestSkillBrowser_ShowsSkillNames(t *testing.T) {
	m := newSkillBrowserModel(t)

	view := m.View()
	if len(m.skillCat.Categories) == 0 || len(m.skillCat.Categories[0].Skills) == 0 {
		t.Fatal("catalog should have at least one skill in the first category")
	}
	firstSkill := m.skillCat.Categories[0].Skills[0]
	if !strings.Contains(view, firstSkill.Name) {
		t.Errorf("skill browser should show skill name %q, got:\n%s", firstSkill.Name, view)
	}
}

func TestSkillBrowser_ShowsSkillDescriptions(t *testing.T) {
	m := newSkillBrowserModel(t)

	view := m.View()
	if len(m.skillCat.Categories) == 0 || len(m.skillCat.Categories[0].Skills) == 0 {
		t.Fatal("catalog should have at least one skill in the first category")
	}
	firstSkill := m.skillCat.Categories[0].Skills[0]
	if !strings.Contains(view, firstSkill.Description) {
		t.Errorf("skill browser should show skill description, got:\n%s", view)
	}
}

func TestSkillBrowser_ShowsInstallHint(t *testing.T) {
	m := newSkillBrowserModel(t)

	view := m.View()
	if !strings.Contains(view, "npx skills add") {
		t.Errorf("skill browser should show install hint 'npx skills add', got:\n%s", view)
	}
}

func TestSkillBrowser_ShowsFooterNavigation(t *testing.T) {
	m := newSkillBrowserModel(t)

	view := m.View()
	if !strings.Contains(view, "esc") {
		t.Errorf("skill browser footer should mention 'esc', got:\n%s", view)
	}
}

func TestSkillBrowser_ShowsCursorOnFirstSkill(t *testing.T) {
	m := newSkillBrowserModel(t)
	// scrollIndex = 0 by default — first skill should be highlighted with "> ".
	view := m.View()

	if len(m.skillCat.Categories) == 0 || len(m.skillCat.Categories[0].Skills) == 0 {
		t.Fatal("catalog should have at least one skill")
	}
	firstName := m.skillCat.Categories[0].Skills[0].Name
	if !strings.Contains(view, "> "+firstName) {
		t.Errorf("skill browser should show cursor on first skill '> %s', got:\n%s", firstName, view)
	}
}

// ─── Skill Browser: Navigation ────────────────────────────────────────────────

func TestSkillBrowser_EscReturnsToMenu(t *testing.T) {
	m := newSkillBrowserModel(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)

	if model.screen != screenMenu {
		t.Errorf("esc on skill browser should go to screenMenu (%d), got screen %d",
			screenMenu, model.screen)
	}
}

func TestSkillBrowser_QReturnsToMenu(t *testing.T) {
	m := newSkillBrowserModel(t)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model := updated.(Model)

	if model.screen != screenMenu {
		t.Errorf("q on skill browser should go to screenMenu (%d), got screen %d",
			screenMenu, model.screen)
	}
}

func TestSkillBrowser_RightSwitchesCategory(t *testing.T) {
	m := newSkillBrowserModel(t)
	if len(m.skillCat.Categories) < 2 {
		t.Skip("need at least 2 categories to test switching")
	}
	initialCat := m.skillCatCursor // 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	model := updated.(Model)

	if model.skillCatCursor == initialCat {
		t.Errorf("right key should advance category cursor from %d", initialCat)
	}
}

func TestSkillBrowser_LeftDoesNotGoNegative(t *testing.T) {
	m := newSkillBrowserModel(t)
	m.skillCatCursor = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyLeft})
	model := updated.(Model)

	if model.skillCatCursor != 0 {
		t.Errorf("left key at first category should stay at 0, got %d", model.skillCatCursor)
	}
}

func TestSkillBrowser_RightDoesNotExceedMax(t *testing.T) {
	m := newSkillBrowserModel(t)
	m.skillCatCursor = len(m.skillCat.Categories) - 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	model := updated.(Model)

	want := len(m.skillCat.Categories) - 1
	if model.skillCatCursor != want {
		t.Errorf("right key at last category should stay at %d, got %d", want, model.skillCatCursor)
	}
}

func TestSkillBrowser_TabWrapsCategories(t *testing.T) {
	m := newSkillBrowserModel(t)
	if len(m.skillCat.Categories) == 0 {
		t.Skip("need at least one category")
	}
	// Move to last category, then tab should wrap to 0.
	m.skillCatCursor = len(m.skillCat.Categories) - 1

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	model := updated.(Model)

	if model.skillCatCursor != 0 {
		t.Errorf("tab from last category should wrap to 0, got %d", model.skillCatCursor)
	}
}

func TestSkillBrowser_DownScrollsSkills(t *testing.T) {
	m := newSkillBrowserModel(t)
	if len(m.skillCat.Categories) == 0 || len(m.skillCat.Categories[0].Skills) < 2 {
		t.Skip("need at least 2 skills to test scrolling")
	}
	m.skillScrollIndex = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	model := updated.(Model)

	if model.skillScrollIndex != 1 {
		t.Errorf("down key should advance skillScrollIndex to 1, got %d", model.skillScrollIndex)
	}
}

func TestSkillBrowser_UpDoesNotGoNegative(t *testing.T) {
	m := newSkillBrowserModel(t)
	m.skillScrollIndex = 0

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyUp})
	model := updated.(Model)

	if model.skillScrollIndex != 0 {
		t.Errorf("up key at first skill should stay at 0, got %d", model.skillScrollIndex)
	}
}

func TestSkillBrowser_CategorySwitch_ResetsScrollIndex(t *testing.T) {
	m := newSkillBrowserModel(t)
	if len(m.skillCat.Categories) < 2 {
		t.Skip("need at least 2 categories")
	}
	m.skillScrollIndex = 3 // simulate having scrolled down

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	model := updated.(Model)

	if model.skillScrollIndex != 0 {
		t.Errorf("switching category should reset skillScrollIndex to 0, got %d", model.skillScrollIndex)
	}
}

func TestSkillBrowser_ErrorView_ShowsError(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenSkillBrowser
	m.skillCatErr = fmt.Errorf("catalog unavailable")

	view := m.View()
	if !strings.Contains(view, "catalog unavailable") {
		t.Errorf("skill browser error view should show error message, got:\n%s", view)
	}
}

func TestSkillBrowser_EmptyCatalog_ShowsMessage(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenSkillBrowser
	m.skillCat = skillCatalog{} // zero value: no categories
	m.skillCatErr = nil

	view := m.View()
	if !strings.Contains(view, "No skills found") {
		t.Errorf("skill browser with empty catalog should show 'No skills found', got:\n%s", view)
	}
}

func TestSkillBrowser_InstallHint_IncludesSelectedSkill(t *testing.T) {
	m := newSkillBrowserModel(t)

	view := m.View()
	if len(m.skillCat.Categories) == 0 || len(m.skillCat.Categories[0].Skills) == 0 {
		t.Fatal("catalog should have skills")
	}
	selectedInstall := m.skillCat.Categories[0].Skills[0].Install
	if !strings.Contains(view, "npx skills add -y "+selectedInstall) {
		t.Errorf("install hint should include selected skill install identifier %q, got:\n%s", selectedInstall, view)
	}
}

// ─── Skill Browser: Menu Item Count ──────────────────────────────────────────

func TestMenu_AllItemsIncludingSkills(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu

	view := m.View()
	expected := []string{
		"Init / Setup",
		"Plan (dry-run)",
		"Apply",
		"Team Status",
		"Browse Skills",
		"Verify",
		"Restore Backup",
		"Quit",
	}
	for _, label := range expected {
		if !strings.Contains(view, label) {
			t.Errorf("menu should contain %q, got:\n%s", label, view)
		}
	}
}

// ─── Skill Browser: helper ───────────────────────────────────────────────────

// newSkillBrowserModel returns a Model on screenSkillBrowser with the real
// embedded catalog loaded, failing the test if the catalog cannot be parsed.
func newSkillBrowserModel(t *testing.T) Model {
	t.Helper()
	cat, err := loadSkillCatalog()
	if err != nil {
		t.Fatalf("loadSkillCatalog: %v", err)
	}
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenSkillBrowser
	m.skillCat = cat
	m.skillCatErr = nil
	m.skillCatCursor = 0
	m.skillScrollIndex = 0
	return m
}

// ─── Init Apply Prompt Tests ──────────────────────────────────────────────────

// TestTUI_InitApplyPrompt_ShownAfterInit verifies that when a commandResult
// arrives after a successful init, the model transitions to
// screenInitApplyPrompt and the view contains "Apply now".
func TestTUI_InitApplyPrompt_ShownAfterInit(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenRunning
	m.initJustCompleted = true

	updated, _ := m.Update(commandResult{output: "Init output text", err: nil})
	model := updated.(Model)

	if model.screen != screenInitApplyPrompt {
		t.Errorf("screen = %d, want screenInitApplyPrompt (%d) after successful init", model.screen, screenInitApplyPrompt)
	}
	view := model.View()
	if !strings.Contains(view, "Apply now") {
		t.Errorf("view should contain 'Apply now', got:\n%s", view)
	}
}

// TestTUI_InitApplyPrompt_YRunsApply verifies that pressing 'y' on the apply
// prompt transitions away from screenInitApplyPrompt to screenRunning.
func TestTUI_InitApplyPrompt_YRunsApply(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitApplyPrompt
	m.initOutput = "Init output text"

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model := updated.(Model)

	if model.screen != screenRunning {
		t.Errorf("screen = %d, want screenRunning (%d) after pressing y", model.screen, screenRunning)
	}
	if cmd == nil {
		t.Error("pressing y should dispatch an apply command (non-nil cmd)")
	}
}

// TestTUI_InitApplyPrompt_NShowsResult verifies that pressing 'n' on the apply
// prompt transitions to screenResult with the stored init output.
func TestTUI_InitApplyPrompt_NShowsResult(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitApplyPrompt
	m.initOutput = "Init ran successfully."

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model := updated.(Model)

	if model.screen != screenResult {
		t.Errorf("screen = %d, want screenResult (%d) after pressing n", model.screen, screenResult)
	}
	if !strings.Contains(model.output, "Init ran successfully.") {
		t.Errorf("output should contain init output %q, got %q", "Init ran successfully.", model.output)
	}
}

// TestTUI_InitApplyPrompt_NotShownOnError verifies that when init fails,
// the model goes directly to screenResult without showing the apply prompt.
func TestTUI_InitApplyPrompt_NotShownOnError(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenRunning
	m.initJustCompleted = true

	updated, _ := m.Update(commandResult{output: "", err: fmt.Errorf("init failed: something went wrong")})
	model := updated.(Model)

	if model.screen != screenResult {
		t.Errorf("screen = %d, want screenResult (%d) when init fails", model.screen, screenResult)
	}
	if model.screen == screenInitApplyPrompt {
		t.Error("should NOT show apply prompt when init fails")
	}
}

// ─── Terminal Width Tests ──────────────────────────────────────────────────────

func TestUpdate_WindowSizeMsg(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model := updated.(Model)

	if model.width != 120 {
		t.Errorf("width = %d, want 120", model.width)
	}
	if model.height != 40 {
		t.Errorf("height = %d, want 40", model.height)
	}
	if cmd != nil {
		t.Error("WindowSizeMsg should return nil cmd")
	}
}

func TestPanelWidth_DefaultsTo78(t *testing.T) {
	m := Model{} // width = 0
	if m.panelWidth() != 78 {
		t.Errorf("panelWidth() = %d, want 78 (default when width is 0)", m.panelWidth())
	}
}

func TestPanelWidth_CapsAt120(t *testing.T) {
	m := Model{width: 300}
	if m.panelWidth() != 120 {
		t.Errorf("panelWidth() = %d, want 120 (cap for ultra-wide)", m.panelWidth())
	}
}

func TestPanelWidth_MinimumIs40(t *testing.T) {
	m := Model{width: 20}
	if m.panelWidth() != 40 {
		t.Errorf("panelWidth() = %d, want 40 (minimum)", m.panelWidth())
	}
}

func TestPanelWidth_NormalWidth(t *testing.T) {
	m := Model{width: 100}
	// 100 - 2 (border) = 98
	if m.panelWidth() != 98 {
		t.Errorf("panelWidth() = %d, want 98 for terminal width 100", m.panelWidth())
	}
}

func TestPanelWidth_ExactMinEdge(t *testing.T) {
	// width = 42: 42 - 2 = 40, which equals the minimum — should return 40.
	m := Model{width: 42}
	if m.panelWidth() != 40 {
		t.Errorf("panelWidth() = %d, want 40 for terminal width 42", m.panelWidth())
	}
}

func TestRenderPanel_ContainsContent(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	out := m.renderPanel("hello world")
	if !strings.Contains(out, "hello world") {
		t.Errorf("renderPanel output should contain 'hello world', got: %q", out)
	}
}

// ─── Init Model Tier Screen Tests ─────────────────────────────────────────────

// TestInitModelTier_ScreenExists verifies that the screenInitModelTier constant
// is defined and occupies a unique slot in the screen iota.
func TestInitModelTier_ScreenExists(t *testing.T) {
	// screenInitModelTier must be distinct from other screens.
	screens := []screen{
		screenIntro,
		screenMenu,
		screenRunning,
		screenResult,
		screenInitMethodology,
		screenTeamStatus,
		screenInitMCP,
		screenInitPlugins,
		screenInitScope,
		screenInitAdapters,
		screenInitPreset,
		screenInitInstallSummary,
		screenSkillBrowser,
		screenInitApplyPrompt,
	}
	for _, s := range screens {
		if s == screenInitModelTier {
			t.Errorf("screenInitModelTier (%d) conflicts with another screen constant", screenInitModelTier)
		}
	}
}

// TestInitModelTier_DefaultIsBalanced verifies that NewModel initializes
// modelTier to ModelTierBalanced.
func TestInitModelTier_DefaultIsBalanced(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	if m.modelTier != domain.ModelTierBalanced {
		t.Errorf("modelTier = %q, want %q (default should be balanced)", m.modelTier, domain.ModelTierBalanced)
	}
}

// TestInitModelTier_Enter_AdvancesToMCP verifies that pressing enter on
// the model tier screen stores the selected tier and advances to screenInitMCP.
func TestInitModelTier_Enter_AdvancesToMCP(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitModelTier
	m.setupPreset = domain.PresetCustom
	m.initCursor = 0 // balanced is first

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitMCP {
		t.Errorf("screen = %d, want screenInitMCP (%d) after enter on model tier", model.screen, screenInitMCP)
	}
	if model.modelTier != domain.ModelTierBalanced {
		t.Errorf("modelTier = %q, want %q after selecting first option", model.modelTier, domain.ModelTierBalanced)
	}
}

// ─── screenInitAdapters Tests ─────────────────────────────────────────────────

func TestInitAdapters_ScreenExists(t *testing.T) {
	// screenInitAdapters must be distinct from all other screen constants.
	others := []screen{
		screenIntro,
		screenMenu,
		screenRunning,
		screenResult,
		screenInitMethodology,
		screenTeamStatus,
		screenInitMCP,
		screenInitPlugins,
		screenInitModelTier,
		screenInitScope,
		screenInitPreset,
		screenInitInstallSummary,
		screenSkillBrowser,
		screenInitApplyPrompt,
	}
	for _, s := range others {
		if s == screenInitAdapters {
			t.Errorf("screenInitAdapters (%d) conflicts with screen %d", screenInitAdapters, s)
		}
	}
}

func TestInitPlugins_Enter_GoesToInstallSummary(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitPlugins
	m.pluginSelections = make(map[string]bool)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitInstallSummary {
		t.Errorf("screen = %d, want screenInitInstallSummary (%d) after enter on plugins", model.screen, screenInitInstallSummary)
	}
}

func TestInitAdapters_Esc_GoesToPreset(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitAdapters
	m.agentSelections = make(map[string]bool)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)

	if model.screen != screenInitPreset {
		t.Errorf("screen = %d, want screenInitPreset (%d) after esc on adapters", model.screen, screenInitPreset)
	}
}

func TestInitAdapters_Enter_GoesToMethodology_WhenCustom(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitAdapters
	m.setupPreset = domain.PresetCustom
	m.agentSelections = make(map[string]bool)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitMethodology {
		t.Errorf("screen = %d, want screenInitMethodology (%d) after enter on adapters (custom)", model.screen, screenInitMethodology)
	}
}

func TestInitAdapters_Enter_GoesToMCP_WhenNonCustom(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitAdapters
	m.setupPreset = domain.PresetFullSquad
	m.agentSelections = make(map[string]bool)

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitMCP {
		t.Errorf("screen = %d, want screenInitMCP (%d) after enter on adapters (non-custom)", model.screen, screenInitMCP)
	}
}

func TestInitAdapters_ShowsAllFiveAgents(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitAdapters
	m.agentSelections = make(map[string]bool)

	view := m.View()
	expected := []string{"OpenCode", "Claude Code", "VS Code Copilot", "Cursor", "Windsurf"}
	for _, id := range expected {
		if !strings.Contains(view, id) {
			t.Errorf("adapters view should contain agent %q, got:\n%s", id, view)
		}
	}
}

func TestInitAdapters_OpenCodeCanBeToggled(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, []domain.Adapter{
		&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam},
	}, "/tmp/home")
	m.screen = screenInitAdapters
	m.agentSelections = map[string]bool{
		string(domain.AgentOpenCode): true,
	}
	m.initCursor = 0 // opencode is first

	// Toggle opencode with space — should deselect it.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model := updated.(Model)

	if model.agentSelections[string(domain.AgentOpenCode)] {
		t.Error("toggling opencode should deselect it — it is no longer locked")
	}

	// Toggle again — should re-select it.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model = updated.(Model)

	if !model.agentSelections[string(domain.AgentOpenCode)] {
		t.Error("toggling opencode again should re-select it")
	}
}

func TestInitAdapters_TogglePersonalAdapter(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, []domain.Adapter{
		&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam},
		&mockAdapter{id: domain.AgentCursor, lane: domain.LanePersonal},
	}, "/tmp/home")
	m.screen = screenInitAdapters
	m.agentSelections = map[string]bool{
		string(domain.AgentOpenCode): true,
		string(domain.AgentCursor):   true,
	}
	m.initCursor = 3 // cursor is 4th in the list (index 3)

	// Space toggles cursor off.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model := updated.(Model)

	if model.agentSelections[string(domain.AgentCursor)] {
		t.Error("cursor should be toggled off after space")
	}

	// Space again toggles cursor back on.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model = updated.(Model)

	if !model.agentSelections[string(domain.AgentCursor)] {
		t.Error("cursor should be toggled on again after second space")
	}
}

func TestInitAdapters_AllDetectedPreChecked(t *testing.T) {
	adapters := []domain.Adapter{
		&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam},
		&mockAdapter{id: domain.AgentCursor, lane: domain.LanePersonal},
		&mockAdapter{id: domain.AgentClaudeCode, lane: domain.LanePersonal},
	}
	m := NewModel("1.0.0", domain.ModeTeam, adapters, "/tmp/home")
	m.screen = screenInitPreset
	m.initCursor = 0 // Full Squad

	// Pressing enter on preset → adapters should pre-check detected agents.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitAdapters {
		t.Fatalf("should be on screenInitAdapters, got %d", model.screen)
	}
	if !model.agentSelections[string(domain.AgentOpenCode)] {
		t.Error("opencode should be pre-checked")
	}
	if !model.agentSelections[string(domain.AgentCursor)] {
		t.Error("cursor should be pre-checked (was detected)")
	}
	if !model.agentSelections[string(domain.AgentClaudeCode)] {
		t.Error("claude-code should be pre-checked (was detected)")
	}
}

// ─── screenInitPreset Tests ───────────────────────────────────────────────────

func TestInitPreset_ScreenExists(t *testing.T) {
	others := []screen{
		screenIntro,
		screenMenu,
		screenRunning,
		screenResult,
		screenInitMethodology,
		screenTeamStatus,
		screenInitMCP,
		screenInitPlugins,
		screenInitModelTier,
		screenInitScope,
		screenInitAdapters,
		screenInitInstallSummary,
		screenSkillBrowser,
		screenInitApplyPrompt,
	}
	for _, s := range others {
		if s == screenInitPreset {
			t.Errorf("screenInitPreset (%d) conflicts with screen %d", screenInitPreset, s)
		}
	}
}

func TestInitPreset_FullSquad_SetsMethodologyAndTier(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitPreset
	m.initCursor = 0 // Full Squad is first

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.methodology != domain.MethodologySDD {
		t.Errorf("methodology = %q, want sdd for full-squad", model.methodology)
	}
	if model.modelTier != domain.ModelTierBalanced {
		t.Errorf("modelTier = %q, want balanced for full-squad", model.modelTier)
	}
}

func TestInitPreset_Lean_SetsMethodologyAndTier(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitPreset
	m.initCursor = 1 // Lean is second

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.methodology != domain.MethodologyConventional {
		t.Errorf("methodology = %q, want conventional for lean", model.methodology)
	}
	if model.modelTier != domain.ModelTierStarter {
		t.Errorf("modelTier = %q, want starter for lean", model.modelTier)
	}
}

func TestInitPreset_Custom_GoesToAdapters(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitPreset
	m.initCursor = 2 // Custom is third

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitAdapters {
		t.Errorf("screen = %d, want screenInitAdapters (%d) for custom preset", model.screen, screenInitAdapters)
	}
}

func TestInitPreset_FullSquad_GoesToAdapters(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitPreset
	m.initCursor = 0 // Full Squad

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitAdapters {
		t.Errorf("screen = %d, want screenInitAdapters (%d) for full-squad", model.screen, screenInitAdapters)
	}
}

func TestInitPreset_Esc_GoesToScope(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitPreset

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)

	if model.screen != screenInitScope {
		t.Errorf("screen = %d, want screenInitScope (%d) after esc on preset", model.screen, screenInitScope)
	}
}

// ─── screenInitInstallSummary Tests ──────────────────────────────────────────

func TestInstallSummary_ShowsMethodology(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitInstallSummary
	m.methodology = domain.MethodologySDD
	m.modelTier = domain.ModelTierBalanced
	m.setupPreset = domain.PresetFullSquad

	view := m.View()
	if !strings.Contains(view, "sdd") {
		t.Errorf("install summary should contain methodology 'sdd', got:\n%s", view)
	}
}

func TestInstallSummary_ShowsSelectedAgentsOnly(t *testing.T) {
	adapters := []domain.Adapter{
		&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam},
		&mockAdapter{id: domain.AgentCursor, lane: domain.LanePersonal},
	}
	m := NewModel("1.0.0", domain.ModeTeam, adapters, "/tmp/home")
	m.screen = screenInitInstallSummary
	m.methodology = domain.MethodologySDD
	m.modelTier = domain.ModelTierBalanced
	m.setupPreset = domain.PresetFullSquad
	m.agentSelections = map[string]bool{
		string(domain.AgentOpenCode): true,
		string(domain.AgentCursor):   true,
		// claude-code, vscode-copilot, windsurf not selected
	}

	view := m.View()
	if !strings.Contains(view, "OpenCode") {
		t.Errorf("install summary should show OpenCode, got:\n%s", view)
	}
	if !strings.Contains(view, "Cursor") {
		t.Errorf("install summary should show Cursor (selected), got:\n%s", view)
	}
}

func TestInstallSummary_ShowsPresetName(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitInstallSummary
	m.setupPreset = domain.PresetFullSquad
	m.methodology = domain.MethodologySDD
	m.modelTier = domain.ModelTierBalanced

	view := m.View()
	if !strings.Contains(view, "Full Squad") {
		t.Errorf("install summary should show preset name 'Full Squad', got:\n%s", view)
	}
}

func TestInstallSummary_ShowsModelTier(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitInstallSummary
	m.setupPreset = domain.PresetLean
	m.methodology = domain.MethodologyConventional
	m.modelTier = domain.ModelTierStarter

	view := m.View()
	if !strings.Contains(view, "starter") {
		t.Errorf("install summary should show model tier 'starter', got:\n%s", view)
	}
}

func TestInstallSummary_Enter_GoesToApplyPrompt(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitInstallSummary
	m.methodology = domain.MethodologySDD
	m.modelTier = domain.ModelTierBalanced
	m.setupPreset = domain.PresetFullSquad
	m.mcpSelections = map[string]bool{"context7": true}
	m.pluginSelections = make(map[string]bool)
	// At least one agent must be selected to proceed.
	m.agentSelections = map[string]bool{string(domain.AgentOpenCode): true}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenRunning {
		t.Errorf("screen = %d, want screenRunning (%d) after enter on install summary", model.screen, screenRunning)
	}
	if cmd == nil {
		t.Error("enter on install summary should dispatch a RunInit command")
	}
}

func TestInstallSummary_Esc_GoesToPlugins_WhenCustom(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitInstallSummary
	m.setupPreset = domain.PresetCustom

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)

	if model.screen != screenInitPlugins {
		t.Errorf("screen = %d, want screenInitPlugins (%d) after esc on install summary (custom)", model.screen, screenInitPlugins)
	}
}

func TestInstallSummary_Esc_GoesToMCP_WhenNonCustom(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitInstallSummary
	m.setupPreset = domain.PresetFullSquad

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)

	if model.screen != screenInitMCP {
		t.Errorf("screen = %d, want screenInitMCP (%d) after esc on install summary (non-custom)", model.screen, screenInitMCP)
	}
}

// ─── Empty-selection guard tests ──────────────────────────────────────────────

func TestInstallSummary_EmptySelection_GoesBackToAdapters(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitInstallSummary
	m.methodology = domain.MethodologySDD
	m.modelTier = domain.ModelTierBalanced
	// No agents selected.
	m.agentSelections = make(map[string]bool)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitAdapters {
		t.Errorf("screen = %d, want screenInitAdapters (%d) when no agents selected", model.screen, screenInitAdapters)
	}
	if cmd != nil {
		t.Error("empty selection should not dispatch a RunInit command")
	}
	if !strings.Contains(model.output, "No agents selected") {
		t.Errorf("output should contain 'No agents selected', got: %q", model.output)
	}
}

func TestInstallSummary_NilSelection_GoesBackToAdapters(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitInstallSummary
	m.methodology = domain.MethodologySDD
	// agentSelections is nil (never initialized).

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitAdapters {
		t.Errorf("screen = %d, want screenInitAdapters (%d) when agentSelections is nil", model.screen, screenInitAdapters)
	}
	if cmd != nil {
		t.Error("nil selection should not dispatch a RunInit command")
	}
}

func TestInitAdapters_ViewDoesNotShowRequiredLabel(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, []domain.Adapter{
		&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam},
	}, "/tmp/home")
	m.screen = screenInitAdapters
	m.agentSelections = map[string]bool{string(domain.AgentOpenCode): true}

	view := m.View()
	if strings.Contains(view, "(required)") {
		t.Error("adapters view should NOT contain '(required)' label — OpenCode is no longer locked")
	}
}

func TestInstallSummary_AllDeselectedExceptOne_ProceedsWithOne(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitInstallSummary
	m.methodology = domain.MethodologySDD
	m.modelTier = domain.ModelTierBalanced
	// Only cursor selected.
	m.agentSelections = map[string]bool{
		string(domain.AgentOpenCode): false,
		string(domain.AgentCursor):   true,
	}

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenRunning {
		t.Errorf("screen = %d, want screenRunning (%d) when one agent selected", model.screen, screenRunning)
	}
	if cmd == nil {
		t.Error("one agent selected should dispatch a RunInit command")
	}
}

// ─── MCP Catalog-driven screen ───────────────────────────────────────────────

func TestInitMCP_CatalogShowsAllFiveItems(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMCP
	m.mcpSelections = catalogPreCheckedSelections()

	view := m.View()
	catalog := domain.DefaultMCPCatalog()
	if len(catalog) != 5 {
		t.Fatalf("catalog size changed, expected 5, got %d", len(catalog))
	}
	for _, s := range catalog {
		if !strings.Contains(view, s.Name) {
			t.Errorf("MCP screen missing server %q", s.Name)
		}
		if !strings.Contains(view, s.Description) {
			t.Errorf("MCP screen missing description for %q", s.Name)
		}
	}
}

func TestInitMCP_PreCheckedItemsStartSelected(t *testing.T) {
	sel := catalogPreCheckedSelections()
	catalog := domain.DefaultMCPCatalog()

	for _, s := range catalog {
		got := sel[s.Name]
		if got != s.PreChecked {
			t.Errorf("server %q: selected=%v, want %v (PreChecked)", s.Name, got, s.PreChecked)
		}
	}
}

func TestInitMCP_PreCheckedCount(t *testing.T) {
	sel := catalogPreCheckedSelections()
	var count int
	for _, v := range sel {
		if v {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 pre-checked items, got %d", count)
	}
}

func TestInitMCP_ToggleChangesSelection(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitMCP
	m.mcpSelections = catalogPreCheckedSelections()
	m.initCursor = 0 // context7, which is pre-checked

	// Space should toggle it off.
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model := updated.(Model)
	if model.mcpSelections["context7"] {
		t.Error("toggling pre-checked context7 should deselect it")
	}
}

// ─── Remove Confirmation Screen ───────────────────────────────────────────────

func TestMenuItems_ContainsRemove(t *testing.T) {
	found := false
	for _, item := range menuItems {
		if item.command == "remove" {
			found = true
			break
		}
	}
	if !found {
		t.Error("menuItems should contain a 'remove' entry")
	}
}

func TestMenu_RemoveItemLabel(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu

	view := m.View()
	if !strings.Contains(view, "Remove SquadAI Config") {
		t.Error("menu should show 'Remove SquadAI Config' item")
	}
}

func TestMenu_SelectRemove_NavigatesToConfirmScreen(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu

	// Navigate to the remove item.
	removeIdx := -1
	for i, item := range menuItems {
		if item.command == "remove" {
			removeIdx = i
			break
		}
	}
	if removeIdx < 0 {
		t.Fatal("remove menu item not found")
	}
	m.cursor = removeIdx

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenRemoveConfirm {
		t.Errorf("selecting remove should navigate to screenRemoveConfirm, got %v", model.screen)
	}
}

func TestRemoveConfirm_ViewContainsExpectedText(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenRemoveConfirm
	m.removePreview = "Files to delete (1):\n  AGENTS.md"

	view := m.View()
	if !strings.Contains(view, "Remove SquadAI Config") {
		t.Error("confirmation screen should show title")
	}
	if !strings.Contains(view, "AGENTS.md") {
		t.Error("confirmation screen should show preview content")
	}
	if !strings.Contains(view, "y/Enter") {
		t.Error("confirmation screen should show key hints")
	}
}

func TestRemoveConfirm_EscReturnsToMenu(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenRemoveConfirm

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenMenu {
		t.Errorf("esc on confirm should return to menu, got %v", model.screen)
	}
}

func TestRemoveConfirm_NKeyReturnsToMenu(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenRemoveConfirm

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model := updated.(Model)
	if model.screen != screenMenu {
		t.Errorf("n on confirm should return to menu, got %v", model.screen)
	}
}

func TestRemoveConfirm_YKeyStartsRunning(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenRemoveConfirm

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model := updated.(Model)
	if model.screen != screenRunning {
		t.Errorf("y on confirm should start running, got %v", model.screen)
	}
	if cmd == nil {
		t.Error("y on confirm should dispatch a command")
	}
}

// Suppress unused import warnings.
var _ = fmt.Sprintf
var _ lipgloss.Style

// ─── Post-install auth panel ─────────────────────────────────────────────────

func TestPostInstall_ShowsAuthSetupForGithub(t *testing.T) {
	selections := map[string]bool{
		"github":   true,
		"context7": true,
	}
	panel := renderPostInstallAuthPanel(selections)
	if panel == "" {
		t.Fatal("expected non-empty panel when github is selected")
	}
	if !strings.Contains(panel, "GITHUB_PERSONAL_ACCESS_TOKEN") {
		t.Errorf("panel should mention GITHUB_PERSONAL_ACCESS_TOKEN, got:\n%s", panel)
	}
	if !strings.Contains(panel, "https://github.com/settings/tokens") {
		t.Errorf("panel should include GitHub token URL, got:\n%s", panel)
	}
}

func TestPostInstall_ShowsAuthSetupForSentry(t *testing.T) {
	selections := map[string]bool{"sentry": true}
	panel := renderPostInstallAuthPanel(selections)
	if panel == "" {
		t.Fatal("expected non-empty panel when sentry is selected")
	}
	if !strings.Contains(panel, "SENTRY_AUTH_TOKEN") {
		t.Errorf("panel should mention SENTRY_AUTH_TOKEN, got:\n%s", panel)
	}
	if !strings.Contains(panel, "https://sentry.io/settings/account/api/auth-tokens/") {
		t.Errorf("panel should include Sentry token URL, got:\n%s", panel)
	}
}

func TestPostInstall_HidesAuthPanelWhenNotNeeded(t *testing.T) {
	selections := map[string]bool{
		"context7":            true,
		"memory":              false,
		"sequential-thinking": false,
	}
	panel := renderPostInstallAuthPanel(selections)
	if panel != "" {
		t.Errorf("expected empty panel when only context7 selected, got:\n%s", panel)
	}
}

func TestPostInstall_HidesAuthPanelForEmptySelections(t *testing.T) {
	panel := renderPostInstallAuthPanel(map[string]bool{})
	if panel != "" {
		t.Errorf("expected empty panel for empty selections, got:\n%s", panel)
	}
}

func TestPostInstall_ShowsBothServersWhenBothSelected(t *testing.T) {
	selections := map[string]bool{
		"github": true,
		"sentry": true,
	}
	panel := renderPostInstallAuthPanel(selections)
	if !strings.Contains(panel, "GITHUB_PERSONAL_ACCESS_TOKEN") {
		t.Errorf("panel should mention GitHub token, got:\n%s", panel)
	}
	if !strings.Contains(panel, "SENTRY_AUTH_TOKEN") {
		t.Errorf("panel should mention Sentry token, got:\n%s", panel)
	}
}
