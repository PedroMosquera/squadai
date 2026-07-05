package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// TestMenuSections_Grouping verifies every section carries the agreed items.
func TestMenuSections_Grouping(t *testing.T) {
	want := map[string][]string{
		"Daily":    {"team-status", "apply", "plan", "verify"},
		"Setup":    {"quick-setup", "init", "doctor", "skills"},
		"Advanced": {"watch", "audit", "restore", "remove", "cli-help"},
		"":         {"quit"},
	}
	if len(menuSections) != len(want) {
		t.Fatalf("expected %d sections, got %d", len(want), len(menuSections))
	}
	for _, s := range menuSections {
		wantCmds, ok := want[s.title]
		if !ok {
			t.Errorf("unexpected section %q", s.title)
			continue
		}
		var gotCmds []string
		for _, it := range s.items {
			gotCmds = append(gotCmds, it.command)
		}
		if strings.Join(gotCmds, ",") != strings.Join(wantCmds, ",") {
			t.Errorf("section %q items = %v, want %v", s.title, gotCmds, wantCmds)
		}
	}
}

// TestMenu_EveryCommandReachable walks the cursor over the whole menu and
// asserts every selectable command can be reached and is unique.
func TestMenu_EveryCommandReachable(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu
	m.cursor = firstSelectableMenuRow()

	rows := menuRows()
	reached := map[string]bool{rows[m.cursor].item.command: true}

	model := m
	for i := 0; i < len(rows); i++ {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated.(Model)
		row := rows[model.cursor]
		if !row.selectable() {
			t.Fatalf("cursor landed on header row %d", model.cursor)
		}
		reached[row.item.command] = true
	}

	for _, it := range selectableMenuItems() {
		if !reached[it.command] {
			t.Errorf("menu command %q not reachable by cursor", it.command)
		}
	}
	if len(reached) != len(selectableMenuItems()) {
		t.Errorf("reached %d commands, want %d", len(reached), len(selectableMenuItems()))
	}
}

// TestMenu_SectionHeadersNotSelectable ensures headers never carry the cursor
// marker in the rendered view.
func TestMenu_SectionHeadersNotSelectable(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu

	view := m.View()
	for _, header := range []string{"> Daily", "> Setup", "> Advanced"} {
		if strings.Contains(view, header) {
			t.Errorf("section header rendered as selectable: %q", header)
		}
	}
}

// TestMenu_SelectAdvancedSetup_SkipsScopeScreen verifies the dead scope screen
// is gone: Advanced Setup goes straight to the preset screen.
func TestMenu_SelectAdvancedSetup_SkipsScopeScreen(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.screen = screenMenu
	m.cursor = menuCursorFor(t, "init")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenInitPreset {
		t.Errorf("Advanced Setup should go straight to the preset screen, got %d", model.screen)
	}
}

// TestMenu_SelectQuickSetup_StartsQuickTrack verifies the Quick Setup entry.
func TestMenu_SelectQuickSetup_StartsQuickTrack(t *testing.T) {
	adapters := []domain.Adapter{&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam}}
	m := NewModel("1.0.0", domain.ModeTeam, adapters, "/tmp/home")
	m.screen = screenMenu
	m.cursor = menuCursorFor(t, "quick-setup")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenQuickAgents {
		t.Errorf("Quick Setup should start the quick track, got %d", model.screen)
	}
	if !model.agentSelections[string(domain.AgentOpenCode)] {
		t.Error("detected adapters should be pre-checked when quick setup starts")
	}
}

// TestModelInit_MenuStart_FiresDriftBadge covers the direct-to-menu start.
func TestModelInit_MenuStart_FiresDriftBadge(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	if m.screen != screenMenu {
		t.Fatalf("NewModel should default to the menu screen, got %d", m.screen)
	}
	if m.Init() == nil {
		t.Error("Init should fire the drift badge check when starting on the menu")
	}
}

func TestModelInit_QuickStart_NoDriftBadge(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home").startQuickSetup(false)
	if m.Init() != nil {
		t.Error("Init should not fire commands when starting on the quick track")
	}
}
