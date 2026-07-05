package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// TestViewInitProjectMemory_ShowsToggle verifies the project memory screen
// renders the memory heading and toggle indicator in default (enabled) state.
func TestViewInitProjectMemory_ShowsToggle(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemory
	// projectMemoryEnabled is true by default

	view := m.viewInitProjectMemory()

	if !strings.Contains(view, "memory") && !strings.Contains(view, "Memory") {
		t.Errorf("viewInitProjectMemory should mention 'memory', got:\n%s", view)
	}
	// The toggle should show [x] when enabled (activeStyle renders it, plain text in tests).
	if !strings.Contains(view, "[x]") {
		t.Errorf("viewInitProjectMemory should show [x] toggle when enabled, got:\n%s", view)
	}
}

// TestViewInitProjectMemory_ShowsDisabledToggle verifies the unchecked state.
func TestViewInitProjectMemory_ShowsDisabledToggle(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemory
	m.projectMemoryEnabled = false

	view := m.viewInitProjectMemory()

	if !strings.Contains(view, "[ ]") {
		t.Errorf("viewInitProjectMemory should show [ ] toggle when disabled, got:\n%s", view)
	}
}

// TestViewInitProjectMemory_ShowsKeyHints verifies navigation hints are present.
func TestViewInitProjectMemory_ShowsKeyHints(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemory

	view := m.viewInitProjectMemory()

	if !strings.Contains(view, "space") {
		t.Errorf("viewInitProjectMemory should show 'space' hint, got:\n%s", view)
	}
	if !strings.Contains(view, "enter") {
		t.Errorf("viewInitProjectMemory should show 'enter' hint, got:\n%s", view)
	}
	if !strings.Contains(view, "esc") {
		t.Errorf("viewInitProjectMemory should show 'esc' hint, got:\n%s", view)
	}
}

// TestViewInitProjectMemoryScaffold_ShowsPrompt verifies the scaffold confirmation
// screen renders the y/n prompt.
func TestViewInitProjectMemoryScaffold_ShowsPrompt(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemoryScaffold
	m.projectMemoryEnabled = true

	view := m.viewInitProjectMemoryScaffold()

	if !strings.Contains(view, "Scaffold") {
		t.Errorf("viewInitProjectMemoryScaffold should mention 'Scaffold', got:\n%s", view)
	}
	if !strings.Contains(view, "y/n") {
		t.Errorf("viewInitProjectMemoryScaffold should show 'y/n' hint, got:\n%s", view)
	}
	if !strings.Contains(view, "Yes") {
		t.Errorf("viewInitProjectMemoryScaffold should show 'Yes' option, got:\n%s", view)
	}
	if !strings.Contains(view, "No") {
		t.Errorf("viewInitProjectMemoryScaffold should show 'No' option, got:\n%s", view)
	}
}

// TestViewInitProjectMemoryScaffold_NoCheckedByDefault verifies that the
// scaffold prompt starts with "No" selected (scaffold = false by default).
func TestViewInitProjectMemoryScaffold_NoCheckedByDefault(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemoryScaffold
	// projectMemoryScaffold defaults to false — nMark should be "[x]"

	view := m.viewInitProjectMemoryScaffold()

	if !strings.Contains(view, "[x]") {
		t.Errorf("scaffold screen should show [x] on No option by default, got:\n%s", view)
	}
}

// TestInitProjectMemory_SpaceTogglesEnabled verifies space key flips the enabled flag.
func TestInitProjectMemory_SpaceTogglesEnabled(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemory
	// starts enabled

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model := updated.(Model)

	if model.projectMemoryEnabled {
		t.Error("space should disable project memory when it was enabled")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	model = updated.(Model)

	if !model.projectMemoryEnabled {
		t.Error("space should re-enable project memory")
	}
}

// TestInitProjectMemory_EscGoesToPlugins verifies esc goes back to plugins screen
// when using the custom preset (which shows the plugins screen).
func TestInitProjectMemory_EscGoesToPlugins(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemory
	m.setupPreset = domain.PresetCustom

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)

	if model.screen != screenInitPlugins {
		t.Errorf("esc on project memory (custom) should go to screenInitPlugins (%d), got screen %d",
			screenInitPlugins, model.screen)
	}
}

// TestInitProjectMemory_EscGoesToMCP verifies esc goes back to MCP screen
// when using a non-custom preset (no plugins screen shown).
func TestInitProjectMemory_EscGoesToMCP(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemory
	m.setupPreset = domain.PresetFullSquad

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)

	if model.screen != screenInitMCP {
		t.Errorf("esc on project memory (non-custom) should go to screenInitMCP (%d), got screen %d",
			screenInitMCP, model.screen)
	}
}

// TestInitProjectMemory_EnterDisabled_GoesToInstallSummary verifies that when
// memory is disabled, enter goes directly to install summary.
func TestInitProjectMemory_EnterDisabled_GoesToInstallSummary(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemory
	m.projectMemoryEnabled = false

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitInstallSummary {
		t.Errorf("enter with memory disabled should go to screenInitInstallSummary (%d), got screen %d",
			screenInitInstallSummary, model.screen)
	}
}

// TestInitProjectMemory_EnterEnabled_PathExists_GoesToInstallSummary verifies
// that when memory is enabled but path already exists, enter skips scaffold.
func TestInitProjectMemory_EnterEnabled_PathExists_GoesToInstallSummary(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemory
	m.projectMemoryEnabled = true
	m.projectMemoryPathExists = true

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitInstallSummary {
		t.Errorf("enter with memory enabled + path exists should go to screenInitInstallSummary (%d), got screen %d",
			screenInitInstallSummary, model.screen)
	}
}

// TestInitProjectMemory_EnterEnabled_PathMissing_GoesToScaffold verifies that
// when memory is enabled and path doesn't exist, enter shows scaffold prompt.
func TestInitProjectMemory_EnterEnabled_PathMissing_GoesToScaffold(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemory
	m.projectMemoryEnabled = true
	m.projectMemoryPathExists = false

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)

	if model.screen != screenInitProjectMemoryScaffold {
		t.Errorf("enter with memory enabled + path missing should go to screenInitProjectMemoryScaffold (%d), got screen %d",
			screenInitProjectMemoryScaffold, model.screen)
	}
}

// TestInitProjectMemoryScaffold_YGoesToInstallSummary verifies y advances.
func TestInitProjectMemoryScaffold_YGoesToInstallSummary(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemoryScaffold

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	model := updated.(Model)

	if model.screen != screenInitInstallSummary {
		t.Errorf("y on scaffold screen should go to screenInitInstallSummary (%d), got screen %d",
			screenInitInstallSummary, model.screen)
	}
	if !model.projectMemoryScaffold {
		t.Error("y should set projectMemoryScaffold = true")
	}
}

// TestInitProjectMemoryScaffold_NGoesToInstallSummary verifies n advances without scaffold.
func TestInitProjectMemoryScaffold_NGoesToInstallSummary(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemoryScaffold

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	model := updated.(Model)

	if model.screen != screenInitInstallSummary {
		t.Errorf("n on scaffold screen should go to screenInitInstallSummary (%d), got screen %d",
			screenInitInstallSummary, model.screen)
	}
	if model.projectMemoryScaffold {
		t.Error("n should set projectMemoryScaffold = false")
	}
}

// TestInitProjectMemoryScaffold_EscGoesToProjectMemory verifies esc goes back.
func TestInitProjectMemoryScaffold_EscGoesToProjectMemory(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemoryScaffold

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)

	if model.screen != screenInitProjectMemory {
		t.Errorf("esc on scaffold screen should go to screenInitProjectMemory (%d), got screen %d",
			screenInitProjectMemory, model.screen)
	}
}

// TestInitProjectMemoryScaffold_UppercaseYAndN verifies uppercase Y/N also work.
func TestInitProjectMemoryScaffold_UppercaseYAndN(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenInitProjectMemoryScaffold

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Y'}})
	model := updated.(Model)

	if model.screen != screenInitInstallSummary {
		t.Errorf("Y should advance to install summary, got screen %d", model.screen)
	}
	if !model.projectMemoryScaffold {
		t.Error("Y should set projectMemoryScaffold = true")
	}

	m2 := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m2.screen = screenInitProjectMemoryScaffold

	updated2, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
	model2 := updated2.(Model)

	if model2.screen != screenInitInstallSummary {
		t.Errorf("N should advance to install summary, got screen %d", model2.screen)
	}
	if model2.projectMemoryScaffold {
		t.Error("N should set projectMemoryScaffold = false")
	}
}

// TestNewModel_ProjectMemoryEnabledByDefault verifies the default initialization.
func TestNewModel_ProjectMemoryEnabledByDefault(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	if !m.projectMemoryEnabled {
		t.Error("projectMemoryEnabled should be true by default")
	}
	if !m.projectMemoryScaffold {
		t.Error("projectMemoryScaffold should be true by default (scaffold unless declined)")
	}
}
