package tui

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/alexmosquera/agent-manager-pro/internal/cli"
	"github.com/alexmosquera/agent-manager-pro/internal/domain"
)

// screen tracks which screen is active.
type screen int

const (
	screenIntro screen = iota
	screenMenu
	screenRunning
	screenResult
)

// menuItem is a selectable action.
type menuItem struct {
	label   string
	command string // CLI command name
}

var menuItems = []menuItem{
	{label: "Plan (dry-run)", command: "plan"},
	{label: "Apply", command: "apply"},
	{label: "Sync", command: "sync"},
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
			item := menuItems[m.cursor]
			if item.command == "quit" {
				m.quitting = true
				return m, tea.Quit
			}
			if item.command == "restore" {
				// Restore requires an ID — show prompt in output.
				m.output = "Use CLI: agent-manager restore <backup-id>\n\nRestore requires a backup ID argument.\nRun 'agent-manager backup list' to see available backups."
				m.err = nil
				m.screen = screenResult
				return m, nil
			}
			m.screen = screenRunning
			return m, m.runCommand(item.command)
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
