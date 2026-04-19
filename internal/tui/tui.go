package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/cli"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// Ensure badgeDisabledStyle is used (it renders unchecked checkboxes).
var _ = badgeDisabledStyle

// screen tracks which screen is active.
type screen int

const (
	screenIntro screen = iota
	screenMenu
	screenRunning
	screenResult
	screenInitScope // first screen in init wizard: repo vs global
	screenInitMethodology
	screenTeamStatus
	screenInitMCP
	screenInitPlugins
	screenInitModelTier
	screenInitAdapters       // agent selection checkboxes
	screenInitPreset         // setup preset radio (full-squad / lean / custom)
	screenInitInstallSummary // review and confirm before applying
	screenSkillBrowser
	screenInitApplyPrompt    // "Apply now?" prompt after successful init
	screenRemoveConfirm      // "Remove SquadAI config" confirmation screen
	screenClaudeDefaultAgent // "Set orchestrator as default Claude agent?" yes/no
)

// menuItem is a selectable action.
type menuItem struct {
	label       string
	command     string // CLI command name
	description string // shown when item is highlighted
}

var menuItems = []menuItem{
	{label: "Init / Setup", command: "init", description: "Configure agents, MCP servers, and team methodology for this project"},
	{label: "Plan (dry-run)", command: "plan", description: "Preview what files would be created or updated without making changes"},
	{label: "Apply", command: "apply", description: "Write all planned config files to disk (agents, MCP, rules, etc.)"},
	{label: "Team Status", command: "team-status", description: "Show which agents are configured and their team role assignments"},
	{label: "Browse Skills", command: "skills", description: "Explore and install community skills (code review, testing, etc.)"},
	{label: "Verify", command: "verify", description: "Check that all generated files match the expected configuration"},
	{label: "Restore Backup", command: "restore", description: "Restore files from a backup created before apply or remove"},
	{label: "Remove SquadAI Config", command: "remove", description: "Delete all SquadAI-managed files and the .squadai directory"},
	{label: "CLI Commands", command: "cli-help", description: "Show available CLI commands for scripting and CI/CD pipelines"},
	{label: "Quit", command: "quit", description: "Exit SquadAI"},
}

// skillEntry is a single skill in the curated catalog.
type skillEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Source      string `json:"source"`
}

// skillCategory groups related skills.
type skillCategory struct {
	Name   string       `json:"name"`
	Skills []skillEntry `json:"skills"`
}

// skillCatalog is the top-level structure of skills/catalog.json.
type skillCatalog struct {
	Categories     []skillCategory `json:"categories"`
	InstallCommand string          `json:"install_command"`
	SearchCommand  string          `json:"search_command"`
	BrowseURL      string          `json:"browse_url"`
}

// loadSkillCatalog reads and parses the embedded skills/catalog.json.
func loadSkillCatalog() (skillCatalog, error) {
	raw, err := assets.Read("skills/catalog.json")
	if err != nil {
		return skillCatalog{}, fmt.Errorf("load skill catalog: %w", err)
	}
	var cat skillCatalog
	if err := json.Unmarshal([]byte(raw), &cat); err != nil {
		return skillCatalog{}, fmt.Errorf("parse skill catalog: %w", err)
	}
	// Sort categories and their skills deterministically.
	sort.Slice(cat.Categories, func(i, j int) bool {
		return cat.Categories[i].Name < cat.Categories[j].Name
	})
	for ci := range cat.Categories {
		sort.Slice(cat.Categories[ci].Skills, func(i, j int) bool {
			return cat.Categories[ci].Skills[i].Name < cat.Categories[ci].Skills[j].Name
		})
	}
	return cat, nil
}

// Model is the bubbletea model for the TUI.
type Model struct {
	version  string
	mode     domain.OperationalMode
	adapters []domain.Adapter
	homeDir  string

	width  int
	height int

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
	modelTier        domain.ModelTier

	// Agent selection and preset (new screens).
	agentSelections map[string]bool    // key=AgentID string, val=selected
	setupPreset     domain.SetupPreset // "full-squad", "lean", "custom"

	// Skill browser state.
	skillCat         skillCatalog
	skillCatErr      error
	skillCatCursor   int // selected category index
	skillScrollIndex int // first visible skill within the current category

	// Init apply-prompt state.
	initJustCompleted bool   // set before launching init; triggers apply-prompt on success
	initOutput        string // stores init output while on apply-prompt screen

	// Claude default agent prompt state.
	setClaudeDefaultAgent bool // whether to write .claude/settings.json "agent" field

	// Remove confirmation state.
	removePreview string // dry-run output shown on confirmation screen
}

// NewModel creates a TUI model with the given state.
func NewModel(version string, mode domain.OperationalMode, adapters []domain.Adapter, homeDir string) Model {
	return Model{
		version:   version,
		mode:      mode,
		adapters:  adapters,
		homeDir:   homeDir,
		screen:    screenIntro,
		modelTier: domain.ModelTierBalanced,
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case commandResult:
		m.output = msg.output
		m.err = msg.err
		if m.initJustCompleted && msg.err == nil {
			// Init succeeded — offer the apply-now prompt.
			m.initOutput = msg.output
			m.initJustCompleted = false
			m.screen = screenInitApplyPrompt
		} else {
			m.initJustCompleted = false
			m.screen = screenResult
		}
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
				m.screen = screenInitScope
				m.initCursor = 0
				return m, nil
			case "team-status":
				m.screen = screenTeamStatus
				return m, nil
			case "skills":
				cat, err := loadSkillCatalog()
				m.skillCat = cat
				m.skillCatErr = err
				m.skillCatCursor = 0
				m.skillScrollIndex = 0
				m.screen = screenSkillBrowser
				return m, nil
			case "quit":
				m.quitting = true
				return m, tea.Quit
			case "restore":
				// Restore requires an ID — show prompt in output.
				m.output = "Use CLI: squadai restore <backup-id>\n\nRestore requires a backup ID argument.\nRun 'squadai backup list' to see available backups."
				m.err = nil
				m.screen = screenResult
				return m, nil
			case "remove":
				// Build a dry-run preview to show on the confirmation screen.
				m.removePreview = buildRemovePreview(m.homeDir)
				m.screen = screenRemoveConfirm
				return m, nil
			case "cli-help":
				m.output = cliHelpText()
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

	case screenInitScope:
		switch key {
		case "up", "k":
			if m.initCursor > 0 {
				m.initCursor--
			}
		case "down", "j":
			if m.initCursor < 1 {
				m.initCursor++
			}
		case "enter":
			m.initCursor = 0
			m.screen = screenInitPreset
			return m, nil
		case "esc":
			m.screen = screenMenu
			return m, nil
		}

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
			m.initCursor = 0
			m.screen = screenInitModelTier
			return m, nil
		case "esc":
			m.initCursor = 0
			m.screen = screenInitAdapters
			return m, nil
		}

	case screenTeamStatus:
		switch key {
		case "esc", "enter", "q":
			m.screen = screenMenu
			return m, nil
		}

	case screenInitMCP:
		catalog := domain.DefaultMCPCatalog()
		switch key {
		case "up", "k":
			if m.initCursor > 0 {
				m.initCursor--
			}
		case "down", "j":
			if m.initCursor < len(catalog)-1 {
				m.initCursor++
			}
		case " ":
			if len(catalog) > 0 && m.mcpSelections != nil {
				name := catalog[m.initCursor].Name
				m.mcpSelections[name] = !m.mcpSelections[name]
			}
		case "enter":
			m.initCursor = 0
			if m.setupPreset == domain.PresetCustom {
				m.pluginSelections = make(map[string]bool)
				m.screen = screenInitPlugins
			} else {
				m.screen = screenInitInstallSummary
			}
			return m, nil
		case "esc":
			m.initCursor = 0
			if m.setupPreset == domain.PresetCustom {
				m.screen = screenInitModelTier
			} else {
				m.screen = screenInitAdapters
			}
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
			m.screen = screenInitInstallSummary
			return m, nil
		case "esc":
			m.initCursor = 0
			m.screen = screenInitMCP
			return m, nil
		}

	case screenInitModelTier:
		tiers := []domain.ModelTier{
			domain.ModelTierBalanced,
			domain.ModelTierPerformance,
			domain.ModelTierStarter,
			domain.ModelTierManual,
		}
		switch key {
		case "up", "k":
			if m.initCursor > 0 {
				m.initCursor--
			}
		case "down", "j":
			if m.initCursor < len(tiers)-1 {
				m.initCursor++
			}
		case "enter":
			m.modelTier = tiers[m.initCursor]
			// Initialize MCP selections from catalog pre-checked defaults.
			m.mcpSelections = catalogPreCheckedSelections()
			m.initCursor = 0
			m.screen = screenInitMCP
			return m, nil
		case "esc":
			m.initCursor = 0
			m.screen = screenInitMethodology
			return m, nil
		}

	case screenInitAdapters:
		// All 5 canonical agents in fixed order.
		allAgents := []domain.AgentID{
			domain.AgentOpenCode,
			domain.AgentClaudeCode,
			domain.AgentVSCodeCopilot,
			domain.AgentCursor,
			domain.AgentWindsurf,
		}
		switch key {
		case "up", "k":
			if m.initCursor > 0 {
				m.initCursor--
			}
		case "down", "j":
			if m.initCursor < len(allAgents)-1 {
				m.initCursor++
			}
		case " ":
			// Toggle selection.
			if m.initCursor < len(allAgents) {
				id := string(allAgents[m.initCursor])
				if m.agentSelections == nil {
					m.agentSelections = make(map[string]bool)
				}
				m.agentSelections[id] = !m.agentSelections[id]
			}
		case "enter":
			m.initCursor = 0
			// If Claude Code is selected, ask about default agent before proceeding.
			if m.agentSelections != nil && m.agentSelections[string(domain.AgentClaudeCode)] {
				m.setClaudeDefaultAgent = true // default: yes
				m.screen = screenClaudeDefaultAgent
			} else if m.setupPreset == domain.PresetCustom {
				m.screen = screenInitMethodology
			} else {
				// Non-custom presets: initialize MCP selections from catalog pre-checked defaults.
				m.mcpSelections = catalogPreCheckedSelections()
				m.screen = screenInitMCP
			}
			return m, nil
		case "esc":
			m.initCursor = 0
			m.screen = screenInitPreset
			return m, nil
		}

	case screenInitPreset:
		presets := []domain.SetupPreset{
			domain.PresetFullSquad,
			domain.PresetLean,
			domain.PresetCustom,
		}
		switch key {
		case "up", "k":
			if m.initCursor > 0 {
				m.initCursor--
			}
		case "down", "j":
			if m.initCursor < len(presets)-1 {
				m.initCursor++
			}
		case "enter":
			m.setupPreset = presets[m.initCursor]
			switch m.setupPreset {
			case domain.PresetFullSquad:
				m.methodology = domain.MethodologySDD
				m.modelTier = domain.ModelTierBalanced
			case domain.PresetLean:
				m.methodology = domain.MethodologyConventional
				m.modelTier = domain.ModelTierStarter
			}
			// All presets: initialize agentSelections from detected adapters (pre-checked).
			m.agentSelections = make(map[string]bool)
			for _, a := range m.adapters {
				m.agentSelections[string(a.ID())] = true
			}
			m.initCursor = 0
			m.screen = screenInitAdapters
			return m, nil
		case "esc":
			m.initCursor = 0
			m.screen = screenInitScope
			return m, nil
		}

	case screenInitInstallSummary:
		switch key {
		case "enter":
			// Guard: do not proceed if no agents are selected.
			var selectedAgents []string
			for id, selected := range m.agentSelections {
				if selected {
					selectedAgents = append(selectedAgents, id)
				}
			}
			if len(selectedAgents) == 0 {
				m.output = "No agents selected. Nothing to configure."
				m.screen = screenInitAdapters
				return m, nil
			}
			m.screen = screenRunning
			m.output = ""
			m.err = nil
			m.initJustCompleted = true
			args := []string{"--methodology=" + string(m.methodology)}
			// Add model tier.
			args = append(args, "--model-tier="+string(m.modelTier))
			// Add MCP selections.
			var mcpKeys []string
			for k, selected := range m.mcpSelections {
				if selected {
					mcpKeys = append(mcpKeys, k)
				}
			}
			if len(mcpKeys) > 0 {
				sort.Strings(mcpKeys)
				args = append(args, "--mcp="+strings.Join(mcpKeys, ","))
			}
			// Add plugin selections.
			var pluginKeys []string
			for k, selected := range m.pluginSelections {
				if selected {
					pluginKeys = append(pluginKeys, k)
				}
			}
			if len(pluginKeys) > 0 {
				sort.Strings(pluginKeys)
				args = append(args, "--plugins="+strings.Join(pluginKeys, ","))
			}
			// Add preset (when not custom).
			if m.setupPreset != "" && m.setupPreset != domain.PresetCustom {
				args = append(args, "--preset="+string(m.setupPreset))
			}
			// Add agent filter only when user deselected at least one agent.
			if m.agentSelections != nil {
				var selectedIDs []string
				for id, sel := range m.agentSelections {
					if sel {
						selectedIDs = append(selectedIDs, id)
					}
				}
				// Check if all canonical agents are selected; if not, pass --agents=.
				allCanonical := []string{
					string(domain.AgentOpenCode),
					string(domain.AgentClaudeCode),
					string(domain.AgentVSCodeCopilot),
					string(domain.AgentCursor),
					string(domain.AgentWindsurf),
				}
				selectedSet := make(map[string]bool, len(selectedIDs))
				for _, id := range selectedIDs {
					selectedSet[id] = true
				}
				allSelected := true
				for _, id := range allCanonical {
					if !selectedSet[id] {
						allSelected = false
						break
					}
				}
				if !allSelected && len(selectedIDs) > 0 {
					sort.Strings(selectedIDs)
					args = append(args, "--agents="+strings.Join(selectedIDs, ","))
				}
			}
			// Pass claude default agent flag when opted in.
			if m.setClaudeDefaultAgent {
				args = append(args, "--set-claude-default-agent")
			}
			return m, func() tea.Msg {
				var buf bytes.Buffer
				err := cli.RunInit(args, &buf)
				return commandResult{output: buf.String(), err: err}
			}
		case "esc":
			if m.setupPreset == domain.PresetCustom {
				m.screen = screenInitPlugins
			} else {
				m.screen = screenInitMCP
			}
			return m, nil
		}

	case screenInitApplyPrompt:
		switch key {
		case "y", "enter":
			m.screen = screenRunning
			m.output = ""
			m.err = nil
			return m, m.runCommand("apply")
		case "n", "esc":
			// Show the stored init output on the result screen.
			m.output = m.initOutput
			m.initOutput = ""
			m.screen = screenResult
			return m, nil
		}

	case screenSkillBrowser:
		switch key {
		case "left", "h":
			if m.skillCatCursor > 0 {
				m.skillCatCursor--
				m.skillScrollIndex = 0
			}
		case "right", "l":
			if len(m.skillCat.Categories) > 0 && m.skillCatCursor < len(m.skillCat.Categories)-1 {
				m.skillCatCursor++
				m.skillScrollIndex = 0
			}
		case "tab":
			if len(m.skillCat.Categories) > 0 {
				m.skillCatCursor = (m.skillCatCursor + 1) % len(m.skillCat.Categories)
				m.skillScrollIndex = 0
			}
		case "up", "k":
			if m.skillScrollIndex > 0 {
				m.skillScrollIndex--
			}
		case "down", "j":
			if len(m.skillCat.Categories) > 0 && m.skillCatCursor < len(m.skillCat.Categories) {
				skills := m.skillCat.Categories[m.skillCatCursor].Skills
				if m.skillScrollIndex < len(skills)-1 {
					m.skillScrollIndex++
				}
			}
		case "esc", "q":
			m.screen = screenMenu
			return m, nil
		}

	case screenRemoveConfirm:
		switch key {
		case "y", "enter":
			m.screen = screenRunning
			m.output = ""
			m.err = nil
			return m, func() tea.Msg {
				var buf bytes.Buffer
				err := cli.RunRemove([]string{"--force"}, &buf)
				return commandResult{output: buf.String(), err: err}
			}
		case "n", "esc":
			m.screen = screenMenu
			return m, nil
		}

	case screenClaudeDefaultAgent:
		switch key {
		case "y", "enter":
			m.setClaudeDefaultAgent = true
			m.initCursor = 0
			if m.setupPreset == domain.PresetCustom {
				m.screen = screenInitMethodology
			} else {
				m.mcpSelections = catalogPreCheckedSelections()
				m.screen = screenInitMCP
			}
			return m, nil
		case "n":
			m.setClaudeDefaultAgent = false
			m.initCursor = 0
			if m.setupPreset == domain.PresetCustom {
				m.screen = screenInitMethodology
			} else {
				m.mcpSelections = catalogPreCheckedSelections()
				m.screen = screenInitMCP
			}
			return m, nil
		case "esc":
			m.initCursor = 0
			m.screen = screenInitAdapters
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
			applyArgs := []string{}
			if m.setClaudeDefaultAgent {
				applyArgs = append(applyArgs, "--set-claude-default-agent")
			}
			err = cli.RunApply(applyArgs, &buf)
		case "verify":
			err = cli.RunVerify(nil, &buf)
		}

		return commandResult{output: buf.String(), err: err}
	}
}

// panelWidth returns the width for panel content, accounting for border and padding.
// Falls back to 78 when terminal size is unknown (tests, no TTY).
func (m Model) panelWidth() int {
	if m.width <= 0 {
		return 78 // default 80 - 2 for border
	}
	w := m.width - 2 // subtract border characters
	if w < 40 {
		return 40
	}
	if w > 120 {
		return 120 // cap for readability on ultra-wide terminals
	}
	return w
}

// renderPanel wraps content in the panel style at the current terminal width.
func (m Model) renderPanel(content string) string {
	return panelStyle.Width(m.panelWidth()).Render(content)
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
	case screenInitScope:
		return m.viewInitScope()
	case screenTeamStatus:
		return m.viewTeamStatus()
	case screenInitMCP:
		return m.viewInitMCP()
	case screenInitPlugins:
		return m.viewInitPlugins()
	case screenInitModelTier:
		return m.viewInitModelTier()
	case screenInitAdapters:
		return m.viewInitAdapters()
	case screenInitPreset:
		return m.viewInitPreset()
	case screenInitInstallSummary:
		return m.viewInitInstallSummary()
	case screenSkillBrowser:
		return m.viewSkillBrowser()
	case screenInitApplyPrompt:
		return m.viewInitApplyPrompt()
	case screenRemoveConfirm:
		return m.viewRemoveConfirm()
	case screenClaudeDefaultAgent:
		return m.viewClaudeDefaultAgent()
	}
	return ""
}

func (m Model) viewIntro() string {
	var b strings.Builder

	// Header panel: version + mode
	var hdr strings.Builder
	hdr.WriteString(titleStyle.Render("SquadAI") + "\n")
	hdr.WriteString(fmt.Sprintf("SquadAI %s\n", m.version))
	hdr.WriteString("Team-consistent AI setup with safe local customization.\n")
	hdr.WriteString(fmt.Sprintf("Mode: %s", m.mode))
	b.WriteString(m.renderPanel(hdr.String()))
	b.WriteString("\n\n")

	// Adapter list panel
	b.WriteString(headingStyle.Render("Detected Agents"))
	b.WriteString("\n")
	var adapterContent strings.Builder
	if len(m.adapters) == 0 {
		adapterContent.WriteString("  (none detected)")
	} else {
		for _, a := range m.adapters {
			name := agentDisplayName(a.ID())
			adapterContent.WriteString(fmt.Sprintf("  %s\n", name))
		}
	}
	b.WriteString(m.renderPanel(strings.TrimRight(adapterContent.String(), "\n")))
	b.WriteString("\n\n")

	b.WriteString(mutedStyle.Render("Press any key to continue."))
	return b.String()
}

func (m Model) viewMenu() string {
	var menuContent strings.Builder
	menuContent.WriteString(headingStyle.Render("SquadAI "+m.version) + "\n\n")

	for i, item := range menuItems {
		if i == m.cursor {
			menuContent.WriteString(activeStyle.Render("> "+item.label) + "\n")
		} else {
			menuContent.WriteString("  " + item.label + "\n")
		}
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(menuContent.String(), "\n")))
	b.WriteString("\n\n")

	// Show description of highlighted item.
	if m.cursor >= 0 && m.cursor < len(menuItems) {
		b.WriteString(mutedStyle.Render(menuItems[m.cursor].description))
		b.WriteString("\n")
	}

	b.WriteString(mutedStyle.Render("↑/↓: navigate  enter: select  q: quit"))
	return b.String()
}

func (m Model) viewRunning() string {
	return mutedStyle.Render("Running...") + "\n"
}

func (m Model) viewResult() string {
	var resultContent strings.Builder

	if m.output != "" {
		resultContent.WriteString(m.output)
		if !strings.HasSuffix(m.output, "\n") {
			resultContent.WriteString("\n")
		}
	}
	if m.err != nil {
		resultContent.WriteString("\n")
		resultContent.WriteString(errorStyle.Render("Error: " + m.err.Error()))
		resultContent.WriteString("\n")
	} else if m.output != "" {
		resultContent.WriteString(successStyle.Render("Done."))
		resultContent.WriteString("\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(resultContent.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("Press any key to return to menu."))
	return b.String()
}

// viewInitScope renders the scope selection screen (This Repo vs Global).
func (m Model) viewInitScope() string {
	scopes := []struct {
		label string
		desc  string
	}{
		{"This Repo", "Configure agents for the current project only"},
		{"Global", "Configure agents globally for all projects"},
	}

	var content strings.Builder
	content.WriteString(headingStyle.Render("Setup Scope") + "\n\n")
	content.WriteString("Where do you want to apply the configuration?\n\n")

	for i, s := range scopes {
		if i == m.initCursor {
			content.WriteString(activeStyle.Render("> "+s.label) + "  " + activeStyle.Render(s.desc) + "\n")
		} else {
			content.WriteString("  " + s.label + "  " + s.desc + "\n")
		}
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("↑/↓: navigate   enter: select   esc: back"))
	return b.String()
}

// viewInitMethodology renders the methodology selection screen.
func (m Model) viewInitMethodology() string {
	methodologies := []struct {
		value domain.Methodology
		label string
		desc  string
	}{
		{domain.MethodologyTDD, "TDD", "Test-Driven Development"},
		{domain.MethodologySDD, "SDD", "Spec-Driven Development"},
		{domain.MethodologyConventional, "Conventional", "Standard development workflow"},
	}

	var content strings.Builder
	content.WriteString(headingStyle.Render("Select Development Methodology") + "\n\n")

	for i, m2 := range methodologies {
		// Build the role pipeline line from DefaultTeam
		team := domain.DefaultTeam(m2.value)
		roleNames := sortedKeys(team)

		if i == m.initCursor {
			content.WriteString(activeStyle.Render("> "+m2.label+" — "+m2.desc) + "\n")
		} else {
			content.WriteString("  " + m2.label + " — " + m2.desc + "\n")
		}
		if len(roleNames) > 0 {
			content.WriteString(mutedStyle.Render("    "+strings.Join(roleNames, " → ")) + "\n")
		}
		content.WriteString("\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("↑/↓: navigate  enter: select  esc: back"))
	return b.String()
}

// viewTeamStatus renders the team status screen showing the configured team composition.
func (m Model) viewTeamStatus() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("Team Status") + "\n\n")

	if m.methodology == "" {
		content.WriteString("No methodology selected.\nRun Init to configure your team.\n")
	} else {
		content.WriteString("Methodology: ")
		content.WriteString(methodologyBadgeStyle.Render(string(m.methodology)))
		content.WriteString("\n\n")

		content.WriteString(headingStyle.Render("Team Roles") + "\n")
		team := domain.DefaultTeam(m.methodology)
		names := sortedKeys(team)
		for _, name := range names {
			role := team[name]
			content.WriteString(fmt.Sprintf("  %-14s %s\n", name+":", role.Description))
		}

		// MCP Servers section
		content.WriteString("\n")
		content.WriteString(headingStyle.Render("MCP Servers") + "\n")
		if len(m.mcpSelections) == 0 {
			content.WriteString(mutedStyle.Render("  (not configured)") + "\n")
		} else {
			for _, name := range sortedKeys(m.mcpSelections) {
				if m.mcpSelections[name] {
					content.WriteString("  " + badgeActiveStyle.Render("●") + " " + name + "  " + badgeActiveStyle.Render("active") + "\n")
				}
			}
			allInactive := true
			for _, v := range m.mcpSelections {
				if v {
					allInactive = false
					break
				}
			}
			if allInactive {
				content.WriteString(mutedStyle.Render("  (not configured)") + "\n")
			}
		}

		// Plugins section
		content.WriteString("\n")
		content.WriteString(headingStyle.Render("Plugins") + "\n")
		if len(m.pluginSelections) == 0 {
			content.WriteString(mutedStyle.Render("  (none enabled)") + "\n")
		} else {
			hasEnabled := false
			for _, name := range sortedKeys(m.pluginSelections) {
				if m.pluginSelections[name] {
					content.WriteString("  " + badgeActiveStyle.Render(name) + "\n")
					hasEnabled = true
				}
			}
			if !hasEnabled {
				content.WriteString(mutedStyle.Render("  (none enabled)") + "\n")
			}
		}
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("Press any key to return to menu."))
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

// agentDisplayName returns the proper display name for an agent ID.
func agentDisplayName(id domain.AgentID) string {
	switch id {
	case domain.AgentOpenCode:
		return "OpenCode"
	case domain.AgentClaudeCode:
		return "Claude Code"
	case domain.AgentVSCodeCopilot:
		return "VS Code Copilot"
	case domain.AgentCursor:
		return "Cursor"
	case domain.AgentWindsurf:
		return "Windsurf"
	default:
		return string(id)
	}
}

// cliHelpText returns the CLI commands reference text.
func cliHelpText() string {
	return `CLI Commands Reference
═══════════════════════════════════════════════════

  squadai init             Configure agents, MCP, and methodology
  squadai init --json      Output init result as JSON
  squadai init --with-policy  Also create policy.json

  squadai plan             Preview planned changes (dry-run)
  squadai plan --json      Output plan as JSON

  squadai apply            Write all config files to disk (idempotent — re-run to sync)
  squadai apply --json     Output result as JSON

  squadai verify           Check generated files match config
  squadai verify --json    Output result as JSON

  squadai diff             Show unified diff of pending changes

  squadai team-status      Show agent roles and configuration
  squadai team-status --json

  squadai remove --force   Remove all managed files + .squadai/
  squadai remove --dry-run Preview what would be removed

  squadai restore <id>     Restore from a backup
  squadai backup list      List available backups

  squadai validate-policy  Validate .squadai/policy.json

Run 'squadai <command> --help' for detailed usage.`
}

// catalogPreCheckedSelections builds the initial mcpSelections map from the
// curated catalog, pre-selecting all entries that have PreChecked == true.
func catalogPreCheckedSelections() map[string]bool {
	sel := make(map[string]bool)
	for _, s := range domain.DefaultMCPCatalog() {
		sel[s.Name] = s.PreChecked
	}
	return sel
}

// viewInitMCP renders the MCP server configuration screen.
func (m Model) viewInitMCP() string {
	catalog := domain.DefaultMCPCatalog()

	var content strings.Builder
	content.WriteString(headingStyle.Render("MCP Servers") + "\n\n")

	for i, server := range catalog {
		var checked string
		if m.mcpSelections != nil && m.mcpSelections[server.Name] {
			checked = badgeActiveStyle.Render("[x]")
		} else {
			checked = badgeDisabledStyle.Render("[ ]")
		}

		var nameStr string
		if i == m.initCursor {
			nameStr = activeStyle.Render(server.Name)
		} else {
			nameStr = server.Name
		}

		content.WriteString(fmt.Sprintf("  %s %s\n", checked, nameStr))
		desc := server.Description
		if server.RequiresAuth {
			desc += "  " + authBadgeStyle.Render("(requires setup)")
		}
		content.WriteString(mutedStyle.Render("      "+desc) + "\n")
		content.WriteString("\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("↑/↓: navigate  space: toggle  enter: next  esc: back"))
	return b.String()
}

// viewInitPlugins renders the plugin selection screen.
func (m Model) viewInitPlugins() string {
	filtered := cli.FilterPlugins(cli.AvailablePlugins(), m.adapters, m.methodology)
	names := sortedKeys(filtered)

	var content strings.Builder
	content.WriteString(headingStyle.Render("Optional Plugins") + "\n\n")

	if len(names) == 0 {
		content.WriteString("No plugins available for current configuration.\n")
	} else {
		for i, name := range names {
			plugin := filtered[name]
			var checked string
			if m.pluginSelections != nil && m.pluginSelections[name] {
				checked = badgeActiveStyle.Render("[x]")
			} else {
				checked = badgeDisabledStyle.Render("[ ]")
			}

			var nameStr string
			if i == m.initCursor {
				nameStr = activeStyle.Render(name)
			} else {
				nameStr = name
			}

			content.WriteString(fmt.Sprintf("  %s %s\n", checked, nameStr))
			content.WriteString(fmt.Sprintf("      %s\n", plugin.Description))
			if len(plugin.SupportedAgents) > 0 {
				content.WriteString(mutedStyle.Render("      Supports: "+strings.Join(plugin.SupportedAgents, ", ")) + "\n")
			}
			content.WriteString("\n")
		}
	}

	if m.methodology == domain.MethodologyTDD {
		content.WriteString(errorStyle.Render("⚠ Superpowers plugin is not available with TDD methodology.") + "\n\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("↑/↓: navigate  space: toggle  enter: next  esc: back"))
	return b.String()
}

// viewInitModelTier renders the model tier selection screen.
func (m Model) viewInitModelTier() string {
	tiers := []struct {
		value      domain.ModelTier
		label      string
		desc       string
		detailLine string
	}{
		{
			domain.ModelTierBalanced,
			"Balanced",
			"Best cost/quality ratio (recommended)",
			"Sonnet 4 for complex tasks, fast models for simple ones",
		},
		{
			domain.ModelTierPerformance,
			"Performance",
			"Always flagship models — maximum quality, higher cost",
			"Sonnet 4.5 / GPT-4.1 for everything",
		},
		{
			domain.ModelTierStarter,
			"Starter",
			"Capable models at lowest cost — great for learning",
			"Haiku 3.5 / GPT-4.1-mini for everything",
		},
		{
			domain.ModelTierManual,
			"Manual",
			"Configure models yourself — no defaults applied",
			"",
		},
	}

	var content strings.Builder
	content.WriteString(headingStyle.Render("Model Configuration") + "\n\n")
	content.WriteString("Choose a model tier for your AI agents:\n\n")

	for i, tier := range tiers {
		if i == m.initCursor {
			content.WriteString(activeStyle.Render("> "+tier.label) + "  ")
			content.WriteString(activeStyle.Render(tier.desc) + "\n")
		} else {
			content.WriteString("  " + tier.label + "  " + tier.desc + "\n")
		}
		if tier.detailLine != "" {
			content.WriteString(mutedStyle.Render("    "+tier.detailLine) + "\n")
		}
		content.WriteString("\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("↑/↓: navigate   enter: select   esc: back"))
	return b.String()
}

// viewInitAdapters renders the agent selection checkbox screen.
func (m Model) viewInitAdapters() string {
	// All 5 canonical agents in fixed order.
	allAgents := []domain.AgentID{
		domain.AgentOpenCode,
		domain.AgentClaudeCode,
		domain.AgentVSCodeCopilot,
		domain.AgentCursor,
		domain.AgentWindsurf,
	}

	// Build a set of detected agent IDs for quick lookup.
	detectedSet := make(map[string]bool, len(m.adapters))
	for _, a := range m.adapters {
		detectedSet[string(a.ID())] = true
	}

	var content strings.Builder
	content.WriteString(headingStyle.Render("Select Agents to Configure") + "\n\n")

	for i, agentID := range allAgents {
		id := string(agentID)
		isDetected := detectedSet[id]

		// Determine checked state.
		checked := m.agentSelections != nil && m.agentSelections[id]

		var checkStr string
		if checked {
			checkStr = badgeActiveStyle.Render("[x]")
		} else {
			checkStr = badgeDisabledStyle.Render("[ ]")
		}

		// Build name and suffix.
		name := agentDisplayName(agentID)
		var nameStr, suffix string
		if i == m.initCursor {
			nameStr = activeStyle.Render(name)
		} else {
			nameStr = name
		}

		if !isDetected {
			suffix = mutedStyle.Render("(not detected)")
		}

		line := fmt.Sprintf("  %s %-20s %s", checkStr, nameStr, suffix)
		content.WriteString(line + "\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("↑/↓: navigate  space: toggle  enter: next  esc: back"))
	return b.String()
}

// viewInitPreset renders the setup preset selection screen.
func (m Model) viewInitPreset() string {
	presets := []struct {
		value domain.SetupPreset
		title string
		line1 string
		line2 string
	}{
		{
			domain.PresetFullSquad,
			"Full Squad",
			"SDD methodology (8 roles), balanced models, all",
			"components enabled",
		},
		{
			domain.PresetLean,
			"Lean",
			"Conventional (4 roles), starter models, core only",
			"Rules, memory, standards — minimal footprint",
		},
		{
			domain.PresetCustom,
			"Custom",
			"Pick methodology, model tier, and components",
			"Opens the full setup wizard",
		},
	}

	var content strings.Builder
	content.WriteString(headingStyle.Render("Setup Preset") + "\n\n")
	content.WriteString("Choose a setup preset:\n\n")

	for i, p := range presets {
		if i == m.initCursor {
			content.WriteString(activeStyle.Render("> "+p.title) + "  " + activeStyle.Render(p.line1) + "\n")
			content.WriteString("  " + mutedStyle.Render("                    "+p.line2) + "\n")
		} else {
			content.WriteString("  " + p.title + "  " + p.line1 + "\n")
			content.WriteString(mutedStyle.Render("                    "+p.line2) + "\n")
		}
		content.WriteString("\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("↑/↓: navigate   enter: select   esc: back"))
	return b.String()
}

// viewInitInstallSummary renders the review and confirm screen before applying.
func (m Model) viewInitInstallSummary() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("Review and Confirm") + "\n\n")

	// Preset name.
	presetName := string(m.setupPreset)
	switch m.setupPreset {
	case domain.PresetFullSquad:
		presetName = "Full Squad"
	case domain.PresetLean:
		presetName = "Lean"
	case domain.PresetCustom:
		presetName = "Custom"
	case "":
		presetName = "Custom"
	}

	// Methodology display.
	methodDesc := string(m.methodology)
	if m.methodology != "" {
		team := domain.DefaultTeam(m.methodology)
		methodDesc = fmt.Sprintf("%s (%d roles)", m.methodology, len(team))
	}

	// Model tier display.
	tierName := string(m.modelTier)
	if tierName == "" {
		tierName = string(domain.ModelTierBalanced)
	}

	content.WriteString(fmt.Sprintf("  %-16s %s\n", "Preset", presetName))
	if m.methodology != "" {
		content.WriteString(fmt.Sprintf("  %-16s %s\n", "Methodology", methodDesc))
	}
	content.WriteString(fmt.Sprintf("  %-16s %s\n", "Model tier", tierName))

	content.WriteString("\n")
	content.WriteString(headingStyle.Render("Agents") + "\n")

	// List all 5 canonical agents; show only selected ones.
	allAgents := []domain.AgentID{
		domain.AgentOpenCode,
		domain.AgentClaudeCode,
		domain.AgentVSCodeCopilot,
		domain.AgentCursor,
		domain.AgentWindsurf,
	}

	hasAny := false
	for _, agentID := range allAgents {
		id := string(agentID)
		selected := m.agentSelections != nil && m.agentSelections[id]
		if !selected {
			continue
		}
		hasAny = true
		content.WriteString(fmt.Sprintf("  %s\n", agentDisplayName(agentID)))
	}
	if !hasAny {
		content.WriteString(mutedStyle.Render("  (none selected)") + "\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("enter: install   esc: back"))
	return b.String()
}

// renderPostInstallAuthPanel builds a panel listing auth-requiring MCP servers
// that were selected. Returns an empty string when no auth is needed.
func renderPostInstallAuthPanel(mcpSelections map[string]bool) string {
	catalog := domain.DefaultMCPCatalog()

	var authServers []domain.CuratedMCPServer
	for _, s := range catalog {
		if s.RequiresAuth && mcpSelections[s.Name] {
			authServers = append(authServers, s)
		}
	}
	if len(authServers) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString(headingStyle.Render("Post-install setup") + "\n")
	b.WriteString("Some MCPs need credentials before they will work:\n\n")
	for _, s := range authServers {
		b.WriteString(authBadgeStyle.Render("• "+s.Name) + "\n")
		for _, env := range s.AuthEnvVars {
			b.WriteString("  Set:  " + env + "\n")
		}
		if s.SetupURL != "" {
			b.WriteString("  Get:  " + s.SetupURL + "\n")
		}
		if s.SetupHint != "" {
			b.WriteString("  Hint: " + s.SetupHint + "\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// viewInitApplyPrompt renders the "Apply now?" confirmation prompt shown after
// a successful init run.
func (m Model) viewInitApplyPrompt() string {
	var content strings.Builder
	content.WriteString(successStyle.Render("✓ Init completed successfully!") + "\n\n")
	content.WriteString("Apply now to install configuration files? [y/n]\n")

	// Show post-install auth guidance when any selected MCPs require credentials.
	if panel := renderPostInstallAuthPanel(m.mcpSelections); panel != "" {
		content.WriteString("\n")
		content.WriteString(panel)
		content.WriteString("\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("Press y or Enter to apply, n or Esc to view init output."))
	return b.String()
}

// viewSkillBrowser renders the community skill browser screen.
func (m Model) viewSkillBrowser() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("Community Skills (skills.sh)") + "\n\n")

	if m.skillCatErr != nil {
		content.WriteString(errorStyle.Render("Could not load catalog: "+m.skillCatErr.Error()) + "\n")
		var b strings.Builder
		b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("esc/q: back to menu"))
		return b.String()
	}

	if len(m.skillCat.Categories) == 0 {
		content.WriteString(mutedStyle.Render("No skills found in catalog.") + "\n")
		var b strings.Builder
		b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
		b.WriteString("\n\n")
		b.WriteString(mutedStyle.Render("esc/q: back to menu"))
		return b.String()
	}

	// Category tab bar.
	for i, cat := range m.skillCat.Categories {
		if i > 0 {
			content.WriteString("  ")
		}
		if i == m.skillCatCursor {
			content.WriteString(activeStyle.Render("[" + cat.Name + "]"))
		} else {
			content.WriteString(mutedStyle.Render(" " + cat.Name + " "))
		}
	}
	content.WriteString("\n\n")

	// Skills list for the selected category.
	currentCat := m.skillCat.Categories[m.skillCatCursor]
	for si, skill := range currentCat.Skills {
		if si == m.skillScrollIndex {
			content.WriteString(activeStyle.Render("> "+skill.Name) + "\n")
		} else {
			content.WriteString("  " + skill.Name + "\n")
		}
		content.WriteString(mutedStyle.Render("    "+skill.Description) + "\n")
	}

	// Install hint footer.
	content.WriteString("\n")
	installCmd := m.skillCat.InstallCommand
	if installCmd == "" {
		installCmd = "npx skills install"
	}
	browseURL := m.skillCat.BrowseURL
	if browseURL == "" {
		browseURL = "https://skills.sh"
	}
	selectedSkill := ""
	if len(currentCat.Skills) > 0 && m.skillScrollIndex < len(currentCat.Skills) {
		selectedSkill = " " + currentCat.Skills[m.skillScrollIndex].Name
	}
	content.WriteString(mutedStyle.Render(
		"Install: "+installCmd+selectedSkill+"  |  Browse more: "+browseURL,
	) + "\n")

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("tab/←/→: category  ↑/↓: skill  esc/q: back"))
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
	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// buildRemovePreview runs a dry-run of Remove and returns a human-readable
// summary to show on the confirmation screen.
func buildRemovePreview(homeDir string) string {
	projectDir, err := os.Getwd()
	if err != nil {
		return "Could not determine project directory."
	}

	report, err := cli.Remove(cli.RemoveOptions{DryRun: true, ProjectDir: projectDir})
	if err != nil {
		return fmt.Sprintf("Preview unavailable: %v", err)
	}

	if len(report.RemovedFiles) == 0 && len(report.CleanedFiles) == 0 {
		return "No SquadAI-managed files found in this project."
	}

	var b strings.Builder
	if len(report.RemovedFiles) > 0 {
		b.WriteString(fmt.Sprintf("Files to delete (%d):\n", len(report.RemovedFiles)))
		for _, f := range report.RemovedFiles {
			rel, relErr := filepath.Rel(projectDir, f)
			if relErr != nil {
				rel = f
			}
			b.WriteString("  " + rel + "\n")
		}
	}
	if len(report.CleanedFiles) > 0 {
		b.WriteString(fmt.Sprintf("Files to strip markers from (%d):\n", len(report.CleanedFiles)))
		for _, f := range report.CleanedFiles {
			rel, relErr := filepath.Rel(projectDir, f)
			if relErr != nil {
				rel = f
			}
			b.WriteString("  " + rel + " (user content preserved)\n")
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// viewRemoveConfirm renders the remove confirmation screen.
func (m Model) viewRemoveConfirm() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("Remove SquadAI Config") + "\n\n")
	content.WriteString("This will remove all SquadAI-managed configuration from this project.\n\n")

	if m.removePreview != "" {
		content.WriteString(m.removePreview + "\n\n")
	}

	content.WriteString(errorStyle.Render("⚠ This action cannot be undone (a backup is created automatically).") + "\n\n")
	content.WriteString("Confirm removal? [y/n]\n")

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("y/Enter: confirm   n/Esc: cancel"))
	return b.String()
}

// viewClaudeDefaultAgent renders the yes/no prompt for setting the orchestrator
// as the default Claude Code agent via .claude/settings.json.
func (m Model) viewClaudeDefaultAgent() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("Claude Code — Default Agent") + "\n\n")
	content.WriteString("Set orchestrator as the default Claude Code agent?\n\n")
	content.WriteString("This writes .claude/settings.json with:\n")
	content.WriteString(mutedStyle.Render(`  { "agent": "orchestrator" }`) + "\n\n")
	content.WriteString("When set, running `claude` will automatically start the orchestrator\n")
	content.WriteString("without needing `--agent orchestrator` every time.\n\n")

	if m.setClaudeDefaultAgent {
		content.WriteString(activeStyle.Render("> Yes") + "  No\n")
	} else {
		content.WriteString("  Yes  " + activeStyle.Render("> No") + "\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("y/Enter: yes   n: no   esc: back"))
	return b.String()
}
