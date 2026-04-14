package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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
	screenInitMethodology
	screenTeamStatus
	screenInitMCP
	screenInitPlugins
	screenInitModelTier
	screenInitSummary
	screenSkillBrowser
	screenInitApplyPrompt // "Apply now?" prompt after successful init
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
	{label: "Browse Skills", command: "skills"},
	{label: "Verify", command: "verify"},
	{label: "Restore backup", command: "restore"},
	{label: "Quit", command: "quit"},
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

	// Skill browser state.
	skillCat         skillCatalog
	skillCatErr      error
	skillCatCursor   int // selected category index
	skillScrollIndex int // first visible skill within the current category

	// Init apply-prompt state.
	initJustCompleted bool   // set before launching init; triggers apply-prompt on success
	initOutput        string // stores init output while on apply-prompt screen
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
				m.screen = screenInitMethodology
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
			m.screen = screenInitModelTier
			return m, nil
		case "esc":
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
			m.initCursor = 0
			m.screen = screenInitSummary
			return m, nil
		case "esc":
			m.screen = screenInitPlugins
			return m, nil
		}

	case screenInitSummary:
		switch key {
		case "enter":
			m.screen = screenRunning
			m.output = ""
			m.err = nil
			m.initJustCompleted = true
			args := []string{"--methodology=" + string(m.methodology)}
			// Add model tier if not default (always pass it through).
			args = append(args, "--model-tier="+string(m.modelTier))
			// Add MCP selections.
			var mcpKeys []string
			for k, selected := range m.mcpSelections {
				if selected {
					mcpKeys = append(mcpKeys, k)
				}
			}
			if len(mcpKeys) > 0 {
				sort.Strings(mcpKeys) // deterministic
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
				sort.Strings(pluginKeys) // deterministic
				args = append(args, "--plugins="+strings.Join(pluginKeys, ","))
			}
			return m, func() tea.Msg {
				var buf bytes.Buffer
				err := cli.RunInit(args, &buf)
				return commandResult{output: buf.String(), err: err}
			}
		case "esc":
			m.screen = screenInitModelTier
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
	case screenTeamStatus:
		return m.viewTeamStatus()
	case screenInitMCP:
		return m.viewInitMCP()
	case screenInitPlugins:
		return m.viewInitPlugins()
	case screenInitModelTier:
		return m.viewInitModelTier()
	case screenInitSummary:
		return m.viewInitSummary()
	case screenSkillBrowser:
		return m.viewSkillBrowser()
	case screenInitApplyPrompt:
		return m.viewInitApplyPrompt()
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
			id := string(a.ID())
			lane := string(a.Lane())
			strategy := string(a.DelegationStrategy())
			adapterContent.WriteString(fmt.Sprintf("  %-16s %-10s %s\n", id, "("+lane+")", strategy))
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

// viewInitMCP renders the MCP server configuration screen.
func (m Model) viewInitMCP() string {
	mcpServers := cli.DefaultMCPServers()
	names := sortedKeys(mcpServers)

	var content strings.Builder
	content.WriteString(headingStyle.Render("MCP Servers") + "\n\n")

	for i, name := range names {
		server := mcpServers[name]
		var checked, checkedStr string
		if m.mcpSelections != nil && m.mcpSelections[name] {
			checked = badgeActiveStyle.Render("[x]")
			checkedStr = "[x]"
		} else {
			checked = badgeDisabledStyle.Render("[ ]")
			checkedStr = "[ ]"
		}
		_ = checkedStr

		var nameStr string
		if i == m.initCursor {
			nameStr = activeStyle.Render(name)
		} else {
			nameStr = name
		}

		content.WriteString(fmt.Sprintf("  %s %s\n", checked, nameStr))
		if len(server.Command) > 0 {
			content.WriteString(mutedStyle.Render("      "+strings.Join(server.Command, " ")) + "\n")
		}
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

// viewInitSummary renders the init summary confirmation screen.
func (m Model) viewInitSummary() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("Setup Summary") + "\n\n")

	content.WriteString(headingStyle.Render("Methodology") + ": ")
	content.WriteString(methodologyBadgeStyle.Render(string(m.methodology)) + "\n")

	team := domain.DefaultTeam(m.methodology)
	content.WriteString(headingStyle.Render("Team") + fmt.Sprintf(": %d roles\n\n", len(team)))

	content.WriteString(headingStyle.Render("MCP") + ":\n")
	mcpNames := sortedKeys(m.mcpSelections)
	hasMCP := false
	for _, name := range mcpNames {
		if m.mcpSelections[name] {
			content.WriteString(fmt.Sprintf("  ✓ %s\n", name))
			hasMCP = true
		}
	}
	if !hasMCP {
		content.WriteString(mutedStyle.Render("  (none)") + "\n")
	}

	content.WriteString("\n")
	content.WriteString(headingStyle.Render("Plugins") + ":\n")
	hasPlugins := false
	pluginNames := sortedKeys(m.pluginSelections)
	for _, name := range pluginNames {
		if m.pluginSelections[name] {
			content.WriteString(fmt.Sprintf("  ✓ %s\n", name))
			hasPlugins = true
		}
	}
	if !hasPlugins {
		content.WriteString(mutedStyle.Render("  (none)") + "\n")
	}

	content.WriteString("\n")
	content.WriteString(headingStyle.Render("Agents") + ":\n")
	if len(m.adapters) == 0 {
		content.WriteString(mutedStyle.Render("  (none detected)") + "\n")
	} else {
		for _, a := range m.adapters {
			content.WriteString(fmt.Sprintf("  %s\n", a.ID()))
		}
	}

	content.WriteString("\n")
	content.WriteString("This will create:\n")
	content.WriteString(mutedStyle.Render("  .squadai/project.json") + "\n")
	content.WriteString(mutedStyle.Render("  Agent-specific config files (e.g., AGENTS.md, CLAUDE.md)") + "\n")
	content.WriteString(mutedStyle.Render("  .squadai/skills/ directory") + "\n")

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("enter: confirm  esc: go back"))
	return b.String()
}

// viewInitApplyPrompt renders the "Apply now?" confirmation prompt shown after
// a successful init run.
func (m Model) viewInitApplyPrompt() string {
	var content strings.Builder
	content.WriteString(successStyle.Render("✓ Init completed successfully!") + "\n\n")
	content.WriteString("Apply now to install configuration files? [y/n]\n")

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
