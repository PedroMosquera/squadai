package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// newQuickModel returns a model on the quick track with the given adapters.
func newQuickModel(adapters ...domain.Adapter) Model {
	return NewModel("1.0.0", domain.ModePersonal, adapters, "/tmp/home").startQuickSetup(false)
}

// ─── First-run routing ───────────────────────────────────────────────────────

func TestInitialScreen_NoConfig_StartsQuickTrack(t *testing.T) {
	dir := t.TempDir()
	if got := initialScreen(dir); got != screenQuickAgents {
		t.Errorf("initialScreen = %d, want screenQuickAgents (%d)", got, screenQuickAgents)
	}
}

func TestInitialScreen_ConfigExists_StartsMenu(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squadai"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squadai", "project.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := initialScreen(dir); got != screenMenu {
		t.Errorf("initialScreen = %d, want screenMenu (%d)", got, screenMenu)
	}
}

func TestInitialScreen_EmptyCwd_StartsQuickTrack(t *testing.T) {
	if got := initialScreen(""); got != screenQuickAgents {
		t.Errorf("initialScreen(\"\") = %d, want screenQuickAgents", got)
	}
}

// ─── Q1: agents ──────────────────────────────────────────────────────────────

func TestQuickAgents_DetectedPreChecked(t *testing.T) {
	m := newQuickModel(
		&mockAdapter{id: domain.AgentClaudeCode, lane: domain.LanePersonal},
		&mockAdapter{id: domain.AgentCursor, lane: domain.LanePersonal},
	)

	if !m.agentSelections[string(domain.AgentClaudeCode)] {
		t.Error("detected Claude Code should be pre-checked")
	}
	if !m.agentSelections[string(domain.AgentCursor)] {
		t.Error("detected Cursor should be pre-checked")
	}
	if m.agentSelections[string(domain.AgentWindsurf)] {
		t.Error("undetected Windsurf should not be pre-checked")
	}
}

func TestQuickAgents_SpaceTogglesSelection(t *testing.T) {
	m := newQuickModel()
	m.initCursor = 0 // OpenCode

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	model := updated.(Model)
	if !model.agentSelections[string(domain.AgentOpenCode)] {
		t.Error("space should check OpenCode")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeySpace})
	model = updated.(Model)
	if model.agentSelections[string(domain.AgentOpenCode)] {
		t.Error("space should uncheck OpenCode again")
	}
}

func TestQuickAgents_ShowsWelcomeCopy(t *testing.T) {
	m := newQuickModel()
	view := m.View()

	for _, want := range []string{
		"Welcome to SquadAI",
		"Which AI coding tools do you use?",
		"We found these on your machine (pre-checked). Toggle any others you use.",
		"space: toggle   enter: continue   m: full menu   ctrl+c: quit",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("quick agents view missing %q", want)
		}
	}
}

func TestQuickAgents_WideTerminal_ShowsFullBanner(t *testing.T) {
	m := newQuickModel()
	m.width = 100

	view := m.View()
	// A distinctive row of the ASCII wordmark asset.
	if !strings.Contains(view, `|____/`) {
		t.Errorf("wide quick agents view should contain the full ASCII banner, got:\n%s", view)
	}
}

func TestQuickAgents_NarrowTerminal_FallsBackToMonogram(t *testing.T) {
	m := newQuickModel()
	m.width = 45 // panelWidth = 43 < 50

	view := m.View()
	if strings.Contains(view, `|____/`) {
		t.Error("narrow quick agents view should not contain the full ASCII banner")
	}
	if !strings.Contains(view, "▞▀▚") {
		t.Error("narrow quick agents view should fall back to the mini monogram")
	}
}

func TestQuickAgents_EnterWithNoSelection_StaysAndWarns(t *testing.T) {
	m := newQuickModel() // nothing detected, nothing selected

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenQuickAgents {
		t.Errorf("enter with no tools selected should stay on Q1, got screen %d", model.screen)
	}
	if !strings.Contains(model.View(), "Select at least one tool to continue.") {
		t.Error("Q1 should warn when nothing is selected")
	}
}

func TestQuickAgents_EnterAdvancesToStyle(t *testing.T) {
	m := newQuickModel(&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam})

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenQuickStyle {
		t.Errorf("enter should advance to Q2, got screen %d", model.screen)
	}
	if model.initCursor != 1 {
		t.Errorf("Q2 cursor should default to the recommended option, got %d", model.initCursor)
	}
}

// ─── Q2: style ───────────────────────────────────────────────────────────────

func TestQuickStyle_ShowsAllOptions(t *testing.T) {
	m := newQuickModel()
	m.screen = screenQuickStyle

	view := m.View()
	for _, want := range []string{
		"How do you like to work?",
		"Keep it simple",
		"A light setup: sensible rules, low cost, no extra process.",
		"Balanced everyday setup   (recommended)",
		"write tests as they go", // description may be wrapped by the panel
		"Full team of specialists",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("quick style view missing %q", want)
		}
	}
}

func TestQuickStyle_SelectionMapsToPreset(t *testing.T) {
	tests := []struct {
		cursor int
		want   domain.SetupPreset
	}{
		{0, domain.PresetSoloMinimal},
		{1, domain.PresetSoloPower},
		{2, domain.PresetFullSquad},
	}
	for _, tc := range tests {
		m := newQuickModel()
		m.screen = screenQuickStyle
		m.initCursor = tc.cursor

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model := updated.(Model)
		if model.setupPreset != tc.want {
			t.Errorf("cursor %d: preset = %q, want %q", tc.cursor, model.setupPreset, tc.want)
		}
		if model.screen != screenQuickExtras {
			t.Errorf("cursor %d: enter should advance to Q3", tc.cursor)
		}
	}
}

// TestQuickStyle_PresetFlowsIntoInitArgs verifies the style→preset mapping
// lands in the init argument list built by buildInitArgs.
func TestQuickStyle_PresetFlowsIntoInitArgs(t *testing.T) {
	tests := []struct {
		cursor  int
		wantArg string
	}{
		{0, "--preset=solo-minimal"},
		{1, "--preset=solo-power"},
		{2, "--preset=full-squad"},
	}
	for _, tc := range tests {
		m := newQuickModel(&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam})
		m.screen = screenQuickStyle
		m.initCursor = tc.cursor

		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		model := updated.(Model).applyQuickSelections()

		args := model.buildInitArgs()
		if !containsString(args, tc.wantArg) {
			t.Errorf("cursor %d: args %v missing %q", tc.cursor, args, tc.wantArg)
		}
		// The quick track must not pin methodology or model tier explicitly —
		// the preset is the single source of truth.
		for _, a := range args {
			if strings.HasPrefix(a, "--methodology=") || strings.HasPrefix(a, "--model-tier=") {
				t.Errorf("cursor %d: quick track should not pass %q", tc.cursor, a)
			}
		}
	}
}

func TestQuickStyle_EscReturnsToAgents(t *testing.T) {
	m := newQuickModel()
	m.screen = screenQuickStyle

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := updated.(Model)
	if model.screen != screenQuickAgents {
		t.Errorf("esc on Q2 should return to Q1, got %d", model.screen)
	}
}

// ─── Q3: extras ──────────────────────────────────────────────────────────────

func TestQuickExtras_Defaults(t *testing.T) {
	m := newQuickModel()

	if !m.quickExtras["project-memory"] {
		t.Error("project memory should be pre-checked")
	}
	if !m.quickExtras["context7"] {
		t.Error("live documentation lookup should be pre-checked")
	}
	for _, key := range []string{"github", "sentry", "sequential-thinking"} {
		if m.quickExtras[key] {
			t.Errorf("%s should not be pre-checked", key)
		}
	}
}

// TestQuickExtras_MemoryMCPAbsent is the dedup regression test: the community
// knowledge-graph MCP server (config key "memory") must never appear on the
// quick extras screen — Project Memory covers the use case.
func TestQuickExtras_MemoryMCPAbsent(t *testing.T) {
	for _, opt := range quickExtraOptions {
		if !opt.isMemory && opt.key == "memory" {
			t.Fatal("the community memory MCP server must not be a quick extra")
		}
	}

	m := newQuickModel()
	m.screen = screenQuickExtras
	view := m.View()
	if strings.Contains(view, "knowledge-graph") {
		t.Error("quick extras view must not mention the knowledge-graph server")
	}
	if strings.Contains(view, "knowledge graph across sessions") {
		t.Error("quick extras view must not carry the old memory MCP description")
	}
}

func TestQuickExtras_ViewShowsCopy(t *testing.T) {
	m := newQuickModel()
	m.screen = screenQuickExtras

	view := m.View()
	for _, want := range []string{
		"Anything extra?",
		`All optional. You can change these later with "Quick Setup".`,
		"Project memory",
		"A shared project notebook your AI tools read and keep up to date.",
		"Live documentation lookup",
		"Lets your tools read up-to-date docs for the libraries you use.",
		"GitHub access",
		"Error tracking (Sentry)",
		"Step-by-step reasoning helper",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("quick extras view missing %q", want)
		}
	}
}

func TestQuickExtras_SpaceToggles(t *testing.T) {
	m := newQuickModel()
	m.screen = screenQuickExtras
	m.initCursor = 2 // github

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	model := updated.(Model)
	if !model.quickExtras["github"] {
		t.Error("space should check github")
	}
}

func TestQuickExtras_EnterAdvancesToSummary(t *testing.T) {
	m := newQuickModel()
	m.screen = screenQuickExtras

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenQuickSummary {
		t.Errorf("enter on Q3 should advance to Q4, got %d", model.screen)
	}
}

// ─── Q4: summary ─────────────────────────────────────────────────────────────

func TestQuickSummary_ShowsSelections(t *testing.T) {
	m := newQuickModel(&mockAdapter{id: domain.AgentClaudeCode, lane: domain.LanePersonal})
	m.setupPreset = domain.PresetSoloPower
	m.screen = screenQuickSummary

	view := m.View()
	for _, want := range []string{
		"Ready to set up",
		"Claude Code",
		"Balanced everyday setup",
		"Project memory",
		"Claude Code will start with the SquadAI coordinator by default.",
		"This writes config files into this project.",
		"undoes everything.",
		"enter: set everything up   c: customize (advanced)   esc: back",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("quick summary view missing %q, got:\n%s", want, view)
		}
	}
}

func TestQuickSummary_NoClaudeLine_WhenClaudeNotSelected(t *testing.T) {
	m := newQuickModel(&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam})
	m.setupPreset = domain.PresetSoloPower
	m.screen = screenQuickSummary

	if strings.Contains(m.View(), "SquadAI coordinator by default") {
		t.Error("summary should not mention the Claude default agent when Claude is not selected")
	}
}

func TestQuickSummary_EnterRunsInit_WithDistinctMsg(t *testing.T) {
	m := newQuickModel(&mockAdapter{id: domain.AgentClaudeCode, lane: domain.LanePersonal})
	m.setupPreset = domain.PresetSoloPower
	m.screen = screenQuickSummary

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenRunning {
		t.Errorf("enter on Q4 should start running, got %d", model.screen)
	}
	if !model.quickFlow {
		t.Error("enter on Q4 should mark the quick flow active")
	}
	if cmd == nil {
		t.Fatal("enter on Q4 should dispatch the init command")
	}
	if !model.setClaudeDefaultAgent {
		t.Error("selecting Claude on the quick track should opt into the default agent")
	}
}

func TestQuickSummary_ClaudeDefaultAgent_OnlyWhenSelected(t *testing.T) {
	m := newQuickModel(&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam})
	m.setupPreset = domain.PresetSoloPower
	m.screen = screenQuickSummary

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.setClaudeDefaultAgent {
		t.Error("setClaudeDefaultAgent must stay false when Claude Code is not selected")
	}
}

// TestQuickSummary_InitDoneChainsApply verifies the init→apply chain: a
// successful quickInitDoneMsg keeps the model running and dispatches apply.
func TestQuickSummary_InitDoneChainsApply(t *testing.T) {
	m := newQuickModel(&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam})
	m.quickFlow = true
	m.screen = screenRunning

	updated, cmd := m.Update(quickInitDoneMsg{output: "init ok"})
	model := updated.(Model)
	if model.screen != screenRunning {
		t.Errorf("successful init should stay on running while apply chains, got %d", model.screen)
	}
	if cmd == nil {
		t.Fatal("successful init should dispatch the apply command")
	}
	if model.applyProgressCh == nil {
		t.Error("apply progress channel should be wired for live progress")
	}
	if model.initOutput != "init ok" {
		t.Errorf("init output should be retained, got %q", model.initOutput)
	}
}

func TestQuickSummary_InitError_ShowsResult(t *testing.T) {
	m := newQuickModel()
	m.quickFlow = true
	m.screen = screenRunning

	updated, _ := m.Update(quickInitDoneMsg{output: "boom", err: os.ErrPermission})
	model := updated.(Model)
	if model.screen != screenResult {
		t.Errorf("failed init should show the result screen, got %d", model.screen)
	}
	if model.err == nil {
		t.Error("failed init should surface the error")
	}
	if model.quickFlow {
		t.Error("quick flow should end on init failure")
	}
}

func TestQuickFlow_ApplySuccess_ShowsQuickDone(t *testing.T) {
	m := newQuickModel()
	m.quickFlow = true
	m.screen = screenRunning
	m.mcpSelections = map[string]bool{"github": true}

	updated, _ := m.Update(commandResult{output: "applied"})
	model := updated.(Model)
	if model.screen != screenQuickDone {
		t.Errorf("successful apply in quick flow should show the done screen, got %d", model.screen)
	}

	view := model.View()
	if !strings.Contains(view, "✓ You're all set. Open your AI tool in this folder and start working.") {
		t.Error("done screen should show the success message")
	}
	// Post-install auth panel for auth-requiring MCP selections.
	if !strings.Contains(view, "GITHUB_PERSONAL_ACCESS_TOKEN") {
		t.Error("done screen should surface the auth panel for github")
	}
}

func TestQuickFlow_ApplyError_ShowsResult(t *testing.T) {
	m := newQuickModel()
	m.quickFlow = true
	m.screen = screenRunning

	updated, _ := m.Update(commandResult{output: "failed", err: os.ErrPermission})
	model := updated.(Model)
	if model.screen != screenResult {
		t.Errorf("failed apply in quick flow should show the result screen, got %d", model.screen)
	}
}

func TestQuickDone_AnyKeyReturnsToMenu(t *testing.T) {
	m := newQuickModel()
	m.screen = screenQuickDone

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	model := updated.(Model)
	if model.screen != screenMenu {
		t.Errorf("any key on the done screen should return to menu, got %d", model.screen)
	}
}

// ─── Customize (c) → advanced track ─────────────────────────────────────────

func TestQuickSummary_CustomizePreservesAgentSelections(t *testing.T) {
	m := newQuickModel(&mockAdapter{id: domain.AgentClaudeCode, lane: domain.LanePersonal})
	m.agentSelections[string(domain.AgentCursor)] = true // manual extra toggle
	m.setupPreset = domain.PresetSoloPower
	m.screen = screenQuickSummary

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	model := updated.(Model)

	if model.screen != screenInitAdapters {
		t.Errorf("c should switch to the advanced adapters screen, got %d", model.screen)
	}
	if model.setupPreset != domain.PresetCustom {
		t.Errorf("c should switch the preset to custom, got %q", model.setupPreset)
	}
	if !model.agentSelections[string(domain.AgentClaudeCode)] || !model.agentSelections[string(domain.AgentCursor)] {
		t.Error("customize must preserve agent selections")
	}
}

// ─── Re-run from menu passes --merge ─────────────────────────────────────────

func TestQuickSetup_RerunFromMenu_PassesMerge(t *testing.T) {
	m := NewModel("1.0.0", domain.ModePersonal, nil, "/tmp/home")
	m.screen = screenMenu
	m.cursor = menuCursorFor(t, "quick-setup")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model := updated.(Model)
	if model.screen != screenQuickAgents {
		t.Fatalf("quick setup from menu should start Q1, got %d", model.screen)
	}
	if !model.quickMerge {
		t.Fatal("quick setup from menu should run init with --merge")
	}
	model = model.applyQuickSelections()
	if !containsString(model.buildInitArgs(), "--merge") {
		t.Error("buildInitArgs should include --merge on re-run")
	}
}

func TestQuickSetup_FirstRun_NoMerge(t *testing.T) {
	m := newQuickModel().applyQuickSelections()
	if containsString(m.buildInitArgs(), "--merge") {
		t.Error("first-run quick setup should not pass --merge")
	}
}

// ─── buildInitArgs quick-track behavior ──────────────────────────────────────

func TestBuildInitArgs_QuickExtrasMapToMCP(t *testing.T) {
	m := newQuickModel(&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam})
	m.quickExtras["github"] = true
	m.quickExtras["context7"] = true
	m = m.applyQuickSelections()

	args := m.buildInitArgs()
	if !containsString(args, "--mcp=context7,github") {
		t.Errorf("args %v missing --mcp=context7,github", args)
	}
}

// TestBuildInitArgs_NoExtras_PassesMCPNone: with every MCP extra unchecked the
// quick track must pass --mcp=none. Omitting the flag would silently enable
// all recommended servers — including the knowledge-graph one (dedup regression).
func TestBuildInitArgs_NoExtras_PassesMCPNone(t *testing.T) {
	m := newQuickModel(&mockAdapter{id: domain.AgentOpenCode, lane: domain.LaneTeam})
	for k := range m.quickExtras {
		m.quickExtras[k] = false
	}
	m = m.applyQuickSelections()

	args := m.buildInitArgs()
	if !containsString(args, "--mcp=none") {
		t.Errorf("args %v missing --mcp=none", args)
	}
	if !containsString(args, "--without-memory") {
		t.Errorf("args %v missing --without-memory when project memory is unchecked", args)
	}
}

func TestBuildInitArgs_NoMemoryScaffold_WhenDeclined(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.methodology = domain.MethodologyTDD
	m.projectMemoryEnabled = true
	m.projectMemoryScaffold = false

	args := m.buildInitArgs()
	if !containsString(args, "--no-memory-scaffold") {
		t.Errorf("args %v missing --no-memory-scaffold", args)
	}
	if containsString(args, "--without-memory") {
		t.Errorf("args %v must not disable memory when only the scaffold was declined", args)
	}
}

func TestBuildInitArgs_ScaffoldAccepted_NoFlag(t *testing.T) {
	m := NewModel("1.0.0", domain.ModeTeam, nil, "/tmp/home")
	m.methodology = domain.MethodologyTDD

	args := m.buildInitArgs()
	if containsString(args, "--no-memory-scaffold") {
		t.Errorf("args %v should not skip the scaffold by default", args)
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}
