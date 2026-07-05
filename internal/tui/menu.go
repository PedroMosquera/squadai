package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// menuItem is a selectable action in the main menu.
type menuItem struct {
	label       string
	command     string // internal command name dispatched on enter
	description string // shown when item is highlighted
}

// menuSection groups related menu items under a muted header.
type menuSection struct {
	title string // empty title renders no header (used for Quit)
	items []menuItem
}

// menuSections is the grouped main menu: everyday actions first, setup next,
// power-user tools last.
var menuSections = []menuSection{
	{
		title: "Daily",
		items: []menuItem{
			{label: "Team Status", command: "team-status", description: "Show which agents are configured and their team role assignments"},
			{label: "Apply", command: "apply", description: "Write all planned config files to disk (agents, MCP, rules, etc.)"},
			{label: "Plan (dry-run)", command: "plan", description: "Preview what files would be created or updated without making changes"},
			{label: "Verify", command: "verify", description: "Check that all generated files match the expected configuration"},
		},
	},
	{
		title: "Setup",
		items: []menuItem{
			{label: "Quick Setup", command: "quick-setup", description: "Four quick questions that set up all your AI tools in one go"},
			{label: "Advanced Setup", command: "init", description: "Full wizard: methodology, model tier, MCP servers, plugins, and more"},
			{label: "Doctor", command: "doctor", description: "Run pre-flight diagnostics: environment, agents, config, MCP, filesystem"},
			{label: "Browse Skills", command: "skills", description: "Explore and install community skills (code review, testing, etc.)"},
		},
	},
	{
		title: "Advanced",
		items: []menuItem{
			{label: "Watch (drift monitor)", command: "watch", description: "Monitor managed files for drift in real time"},
			{label: "Audit Log", command: "audit", description: "View the governance audit log (.squadai/audit.log)"},
			{label: "Restore Backup", command: "restore", description: "Restore files from a backup created before apply or remove"},
			{label: "Remove SquadAI Config", command: "remove", description: "Delete all SquadAI-managed files and the .squadai directory"},
			{label: "CLI Commands", command: "cli-help", description: "Show available CLI commands for scripting and CI/CD pipelines"},
		},
	},
	{
		title: "",
		items: []menuItem{
			{label: "Quit", command: "quit", description: "Exit SquadAI"},
		},
	},
}

// menuRow is one rendered line of the menu: either a non-selectable section
// header or a selectable item.
type menuRow struct {
	header string   // non-empty for section header rows
	item   menuItem // valid when header == ""
}

// selectable reports whether the cursor may rest on this row.
func (r menuRow) selectable() bool { return r.header == "" }

// menuRows flattens menuSections into a single list of rows.
func menuRows() []menuRow {
	var rows []menuRow
	for _, s := range menuSections {
		if s.title != "" {
			rows = append(rows, menuRow{header: s.title})
		}
		for _, it := range s.items {
			rows = append(rows, menuRow{item: it})
		}
	}
	return rows
}

// selectableMenuItems returns every selectable item in display order.
func selectableMenuItems() []menuItem {
	var items []menuItem
	for _, r := range menuRows() {
		if r.selectable() {
			items = append(items, r.item)
		}
	}
	return items
}

// firstSelectableMenuRow returns the index of the first selectable row.
func firstSelectableMenuRow() int {
	for i, r := range menuRows() {
		if r.selectable() {
			return i
		}
	}
	return 0
}

// nextSelectableMenuRow returns the nearest selectable row index starting at
// `from` and stepping by `dir` (+1 or -1). Returns `current` when no
// selectable row exists in that direction.
func nextSelectableMenuRow(current, dir int) int {
	rows := menuRows()
	for i := current + dir; i >= 0 && i < len(rows); i += dir {
		if rows[i].selectable() {
			return i
		}
	}
	return current
}

// handleMenuKey processes key input on the grouped main menu.
func (m Model) handleMenuKey(key string) (tea.Model, tea.Cmd) {
	rows := menuRows()
	if m.cursor < 0 || m.cursor >= len(rows) || !rows[m.cursor].selectable() {
		m.cursor = firstSelectableMenuRow()
	}

	switch key {
	case "up", "k":
		m.cursor = nextSelectableMenuRow(m.cursor, -1)
	case "down", "j":
		m.cursor = nextSelectableMenuRow(m.cursor, +1)
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "enter":
		selected := rows[m.cursor].item.command
		switch selected {
		case "quick-setup":
			// Re-running quick setup from the menu merges on top of any
			// existing config instead of replacing it.
			return m.startQuickSetup(true), nil
		case "init":
			m.quickMerge = false
			m.screen = screenInitPreset
			m.initCursor = 0
			return m, nil
		case "team-status":
			m.screen = screenTeamStatus
			return m, nil
		case "doctor":
			m.doctorResults = nil
			m.doctorFixMsg = ""
			m.screen = screenDoctor
			return m, m.runDoctorCmd()
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
		case "watch":
			m.watchResults = nil
			m.watchChecking = true
			m.watchLastAt = time.Time{}
			m.screen = screenWatch
			return m, m.runWatchDriftCmd()
		case "audit":
			m.auditEvents = nil
			m.auditErr = nil
			m.auditScroll = 0
			m.screen = screenAudit
			return m, m.loadAuditCmd()
		case "cli-help":
			m.output = cliHelpText()
			m.err = nil
			m.screen = screenResult
			return m, nil
		default:
			m.screen = screenRunning
			m.output = ""
			m.err = nil
			if selected == "apply" {
				return m, m.runApplyWithProgress()
			}
			return m, m.runCommand(selected)
		}
	}
	return m, nil
}

// viewMenu renders the grouped main menu.
func (m Model) viewMenu() string {
	rows := menuRows()
	cursor := m.cursor
	if cursor < 0 || cursor >= len(rows) || !rows[cursor].selectable() {
		cursor = firstSelectableMenuRow()
	}

	var menuContent strings.Builder
	menuContent.WriteString(headingStyle.Render("Main Menu") + "\n")

	for i, row := range rows {
		if row.header != "" {
			menuContent.WriteString("\n" + mutedStyle.Render(row.header) + "\n")
			continue
		}
		if i == cursor {
			menuContent.WriteString(activeStyle.Render("  > "+row.item.label) + "\n")
		} else {
			menuContent.WriteString("    " + row.item.label + "\n")
		}
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(menuContent.String(), "\n")))
	b.WriteString("\n\n")

	// Show description of highlighted item.
	if cursor >= 0 && cursor < len(rows) && rows[cursor].selectable() {
		b.WriteString(mutedStyle.Render(rows[cursor].item.description))
		b.WriteString("\n")
	}

	// Drift badge.
	b.WriteString(m.renderDriftBadge())
	b.WriteString("\n")

	b.WriteString(mutedStyle.Render("↑/↓: navigate  enter: select  q: quit"))
	return b.String()
}

func (m Model) renderDriftBadge() string {
	if !m.driftBadgeReady {
		return mutedStyle.Render("  drift: checking…")
	}
	if m.driftBadgeCount == 0 {
		return successStyle.Render("  ✓ no drift")
	}
	return authBadgeStyle.Render(fmt.Sprintf("  ⚠ %d file(s) drifted — run verify --strict or doctor", m.driftBadgeCount))
}
