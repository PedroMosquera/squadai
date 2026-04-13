package tui

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PedroMosquera/agent-manager-pro/internal/cli"
	"github.com/PedroMosquera/agent-manager-pro/internal/domain"
)

// screen tracks which screen is active.
type screen int

const (
	screenIntro screen = iota
	screenMenu
	screenRunning
	screenResult
	screenInitMethodology
	screenTeamStatus
	screenInitMCP
	screenInitPlugins
	screenInitSummary
)

// menuItem is a selectable action.
type menuItem struct {
	label   string
	command string // CLI command name
}

var menuItems = []menuItem{
	{label: "Init / Setup", command: "init"},
	{label: "Plan (dry-run)", command: "plan"},
	{label: "Apply", command: "apply"},
	{label: "Sync", command: "sync"},
	{label: "Team Status", command: "team-status"},
	{label: "Verify", command: "verify"},
	{label: "Restore backup", command: "restore"},
	{label: "Quit", command: "quit"},
}

// Model is the bubbletea model for the TUI.
type Model struct {
	version  string
	mode     domain.OperationalMode
	adapters []domain.Adapter
	homeDir  string

	screen   screen
	cursor   int
	output   string
	err      error
	quitting bool

	// Init wizard state.
	methodology      domain.Methodology
	initCursor       int
	mcpSelections    map[string]bool
	pluginSelections map[string]bool
}

// NewModel creates a TUI model with the given state.
func NewModel(version string, mode domain.OperationalMode, adapters []domain.Adapter, homeDir string) Model {
	return Model{
		version:  version,
		mode:     mode,
		adapters: adapters,
		homeDir:  homeDir,
		screen:   screenIntro,
	}
}

// commandResult carries the output of a CLI command execution.
type commandResult struct {
	output string
	err    error
}

// Init is the bubbletea init function.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages and key presses.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKey(msg)
	case commandResult:
		m.output = msg.output
		m.err = msg.err
		m.screen = screenResult
		return m, nil
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Global quit shortcuts.
	if key == "ctrl+c" {
		m.quitting = true
		return m, tea.Quit
	}

	switch m.screen {
	case screenIntro:
		// Any key advances to menu.
		m.screen = screenMenu
		return m, nil

	case screenMenu:
		switch key {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(menuItems)-1 {
				m.cursor++
			}
		case "enter":
			selected := menuItems[m.cursor].command
			switch selected {
			case "init":
				m.screen = screenInitMethodology
				m.initCursor = 0
				return m, nil
			case "team-status":
				m.screen = screenTeamStatus
				return m, nil
			case "quit":
				m.quitting = true
				return m, tea.Quit
			case "restore":
				// Restore requires an ID — show prompt in output.
				m.output = "Use CLI: agent-manager restore <backup-id>\n\nRestore requires a backup ID argument.\nRun 'agent-manager backup list' to see available backups."
				m.err = nil
				m.screen = screenResult
				return m, nil
			default:
				m.screen = screenRunning
				m.output = ""
				m.err = nil
				return m, m.runCommand(selected)
			}
		case "q":
			m.quitting = true
			return m, tea.Quit
		}

	case screenResult:
		// Any key returns to menu.
		m.screen = screenMenu
		m.output = ""
		m.err = nil
		return m, nil

	case screenRunning:
		// Ignore input while running.

	case screenInitMethodology:
		methodologies := []domain.Methodology{
			domain.MethodologyTDD,
			domain.MethodologySDD,
			domain.MethodologyConventional,
		}
		switch key {
		case "up", "k":
			if m.initCursor > 0 {
				m.initCursor--
			}
		case "down", "j":
			if m.initCursor < len(methodologies)-1 {
				m.initCursor++
			}
		case "enter":
			m.methodology = methodologies[m.initCursor]
			// Initialize MCP selections with Context7 pre-selected
			m.mcpSelections = map[string]bool{"context7": true}
			m.initCursor = 0
			m.screen = screenInitMCP
			return m, nil
		case "esc":
			m.screen = screenMenu
			return m, nil
		}

	case screenTeamStatus:
		switch key {
		case "esc", "enter", "q":
			m.screen = screenMenu
			return m, nil
		}

	case screenInitMCP:
		mcpServers := cli.DefaultMCPServers()
		mcpNames := sortedKeys(mcpServers)
		switch key {
		case "up", "k":
			if m.initCursor > 0 {
				m.initCursor--
			}
		case "down", "j":
			if m.initCursor < len(mcpNames)-1 {
				m.initCursor++
			}
		case " ":
			if len(mcpNames) > 0 && m.mcpSelections != nil {
				name := mcpNames[m.initCursor]
				m.mcpSelections[name] = !m.mcpSelections[name]
			}
		case "enter":
			m.pluginSelections = make(map[string]bool)
			m.initCursor = 0
			m.screen = screenInitPlugins
			return m, nil
		case "esc":
			m.screen = screenInitMethodology
			return m, nil
		}

	case screenInitPlugins:
		filtered := cli.FilterPlugins(cli.AvailablePlugins(), m.adapters, m.methodology)
		pluginNames := sortedKeys(filtered)
		switch key {
		case "up", "k":
			if m.initCursor > 0 {
				m.initCursor--
			}
		case "down", "j":
			if m.initCursor < len(pluginNames)-1 {
				m.initCursor++
			}
		case " ":
			if len(pluginNames) > 0 && m.pluginSelections != nil {
				name := pluginNames[m.initCursor]
				m.pluginSelections[name] = !m.pluginSelections[name]
			}
		case "enter":
			m.initCursor = 0
			m.screen = screenInitSummary
			return m, nil
		case "esc":
			m.screen = screenInitMCP
			return m, nil
		}

	case screenInitSummary:
		switch key {
		case "enter":
			m.screen = screenRunning
			m.output = ""
			m.err = nil
			// TODO(v2-session4): Pass mcpSelections and pluginSelections to RunInit
			// once CLI flags for MCP/plugin configuration are implemented.
			args := []string{"--methodology=" + string(m.methodology)}
			return m, func() tea.Msg {
				var buf bytes.Buffer
				err := cli.RunInit(args, &buf)
				return commandResult{output: buf.String(), err: err}
			}
		case "esc":
			m.screen = screenInitPlugins
			return m, nil
		}
	}

	return m, nil
}

func (m Model) runCommand(command string) tea.Cmd {
	return func() tea.Msg {
		var buf bytes.Buffer
		var err error

		switch command {
		case "plan":
			err = cli.RunPlan([]string{"--dry-run"}, &buf)
		case "apply":
			err = cli.RunApply(nil, &buf)
		case "sync":
			err = cli.RunSync(nil, &buf)
		case "verify":
			err = cli.RunVerify(nil, &buf)
		}

		return commandResult{output: buf.String(), err: err}
	}
}

// View renders the TUI.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	switch m.screen {
	case screenIntro:
		return m.viewIntro()
	case screenMenu:
		return m.viewMenu()
	case screenRunning:
		return m.viewRunning()
	case screenResult:
		return m.viewResult()
	case screenInitMethodology:
		return m.viewInitMethodology()
	case screenTeamStatus:
		return m.viewTeamStatus()
	case screenInitMCP:
		return m.viewInitMCP()
	case screenInitPlugins:
		return m.viewInitPlugins()
	case screenInitSummary:
		return m.viewInitSummary()
	}
	return ""
}

func (m Model) viewIntro() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("agent-manager %s\n", m.version))
	b.WriteString("Team-consistent AI setup with safe local customization.\n\n")
	b.WriteString(fmt.Sprintf("Mode: %s\n", m.mode))

	b.WriteString("Adapters:\n")
	if len(m.adapters) == 0 {
		b.WriteString("  (none detected)\n")
	} else {
		for _, a := range m.adapters {
			lane := string(a.Lane())
			b.WriteString(fmt.Sprintf("  %s (%s)\n", a.ID(), lane))
		}
	}

	b.WriteString("\nPress any key to continue.")
	return b.String()
}

func (m Model) viewMenu() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("agent-manager %s\n\n", m.version))

	for i, item := range menuItems {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		b.WriteString(fmt.Sprintf("%s%s\n", cursor, item.label))
	}

	b.WriteString("\nUse arrow keys to navigate, enter to select, q to quit.")
	return b.String()
}

func (m Model) viewRunning() string {
	return "Running...\n"
}

func (m Model) viewResult() string {
	var b strings.Builder

	if m.output != "" {
		b.WriteString(m.output)
		if !strings.HasSuffix(m.output, "\n") {
			b.WriteString("\n")
		}
	}
	if m.err != nil {
		b.WriteString(fmt.Sprintf("\nError: %v\n", m.err))
	}

	b.WriteString("\nPress any key to return to menu.")
	return b.String()
}

// viewInitMethodology renders the methodology selection screen.
func (m Model) viewInitMethodology() string {
	var b strings.Builder
	b.WriteString("Select Methodology\n\n")
	methodologies := []struct {
		value domain.Methodology
		label string
		desc  string
	}{
		{domain.MethodologyTDD, "TDD (Test-Driven Development)", "6 roles: red-green-refactor workflow"},
		{domain.MethodologySDD, "SDD (Spec-Driven Development)", "8 roles: specification-first workflow"},
		{domain.MethodologyConventional, "Conventional", "4 roles: standard development workflow"},
	}
	for i, m2 := range methodologies {
		cursor := "  "
		if i == m.initCursor {
			cursor = "> "
		}
		fmt.Fprintf(&b, "%s%s\n    %s\n\n", cursor, m2.label, m2.desc)
	}
	b.WriteString("\n↑/↓: navigate  enter: select  esc: back")
	return b.String()
}

// viewTeamStatus renders the team status screen showing the configured team composition.
func (m Model) viewTeamStatus() string {
	var b strings.Builder
	b.WriteString("Team Status\n\n")
	if m.methodology == "" {
		b.WriteString("No methodology selected.\nRun Init to configure your team.\n")
	} else {
		fmt.Fprintf(&b, "Methodology: %s\n\n", m.methodology)
		team := domain.DefaultTeam(m.methodology)
		names := sortedKeys(team)
		for _, name := range names {
			role := team[name]
			fmt.Fprintf(&b, "  %-14s %s\n", name+":", role.Description)
		}
	}
	b.WriteString("\nPress any key to return to menu.")
	return b.String()
}

// sortedKeys returns the sorted keys of any map[string]V.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// viewInitMCP renders the MCP server configuration screen.
func (m Model) viewInitMCP() string {
	var b strings.Builder
	b.WriteString("MCP Server Configuration\n\n")
	mcpServers := cli.DefaultMCPServers()
	names := sortedKeys(mcpServers)
	for i, name := range names {
		cursor := "  "
		if i == m.initCursor {
			cursor = "> "
		}
		checked := "[ ]"
		if m.mcpSelections != nil && m.mcpSelections[name] {
			checked = "[x]"
		}
		server := mcpServers[name]
		fmt.Fprintf(&b, "%s%s %s\n", cursor, checked, name)
		fmt.Fprintf(&b, "      Type: %s\n\n", server.Type)
	}
	b.WriteString("\n↑/↓: navigate  space: toggle  enter: next  esc: back")
	return b.String()
}

// viewInitPlugins renders the plugin selection screen.
func (m Model) viewInitPlugins() string {
	var b strings.Builder
	b.WriteString("Plugin Selection\n\n")
	filtered := cli.FilterPlugins(cli.AvailablePlugins(), m.adapters, m.methodology)
	names := sortedKeys(filtered)
	if len(names) == 0 {
		b.WriteString("No plugins available for current configuration.\n")
	} else {
		for i, name := range names {
			cursor := "  "
			if i == m.initCursor {
				cursor = "> "
			}
			checked := "[ ]"
			if m.pluginSelections != nil && m.pluginSelections[name] {
				checked = "[x]"
			}
			plugin := filtered[name]
			fmt.Fprintf(&b, "%s%s %s\n", cursor, checked, name)
			fmt.Fprintf(&b, "      %s\n\n", plugin.Description)
		}
	}
	if m.methodology == domain.MethodologyTDD {
		b.WriteString("Note: Superpowers plugin is not available with TDD methodology.\n\n")
	}
	b.WriteString("↑/↓: navigate  space: toggle  enter: next  esc: back")
	return b.String()
}

// viewInitSummary renders the init summary confirmation screen.
func (m Model) viewInitSummary() string {
	var b strings.Builder
	b.WriteString("Init Summary\n\n")
	fmt.Fprintf(&b, "Methodology: %s\n", m.methodology)
	team := domain.DefaultTeam(m.methodology)
	fmt.Fprintf(&b, "Team roles:  %d\n\n", len(team))

	b.WriteString("MCP Servers:\n")
	mcpNames := sortedKeys(m.mcpSelections)
	for _, name := range mcpNames {
		if m.mcpSelections[name] {
			fmt.Fprintf(&b, "  ✓ %s\n", name)
		}
	}

	b.WriteString("\nPlugins:\n")
	hasPlugins := false
	pluginNames := sortedKeys(m.pluginSelections)
	for _, name := range pluginNames {
		if m.pluginSelections[name] {
			fmt.Fprintf(&b, "  ✓ %s\n", name)
			hasPlugins = true
		}
	}
	if !hasPlugins {
		b.WriteString("  (none)\n")
	}

	b.WriteString("\nPress enter to confirm  esc: back")
	return b.String()
}

// Run starts the TUI application.
func Run(version string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("resolve home directory: %w", err)
	}

	// Load config to determine mode.
	merged, err := cli.LoadAndMerge(homeDir, "")
	if err != nil {
		// If config loading fails, use defaults.
		merged = &domain.MergedConfig{
			Mode: domain.ModePersonal,
		}
	}

	adapters := cli.DetectAdapters(homeDir)

	model := NewModel(version, merged.Mode, adapters, homeDir)
	p := tea.NewProgram(model)
	_, err = p.Run()
	return err
}
