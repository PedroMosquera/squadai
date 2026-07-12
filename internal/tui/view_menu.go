package tui

import (
	"fmt"
	"strings"

	"github.com/PedroMosquera/squadai/internal/pipeline"
)

// Ensure badgeDisabledStyle is used (it renders unchecked checkboxes).
var _ = badgeDisabledStyle

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
	{label: "Doctor", command: "doctor", description: "Run pre-flight diagnostics: environment, agents, config, MCP, filesystem"},
	{label: "Browse Skills", command: "skills", description: "Explore and install community skills (code review, testing, etc.)"},
	{label: "Verify", command: "verify", description: "Check that all generated files match the expected configuration"},
	{label: "Restore Backup", command: "restore", description: "Restore files from a backup created before apply or remove"},
	{label: "Remove SquadAI Config", command: "remove", description: "Delete all SquadAI-managed files and the .squadai directory"},
	{label: "Watch (drift monitor)", command: "watch", description: "Monitor managed files for drift in real time"},
	{label: "Audit Log", command: "audit", description: "View the governance audit log (.squadai/audit.log)"},
	{label: "CLI Commands", command: "cli-help", description: "Show available CLI commands for scripting and CI/CD pipelines"},
	{label: "Quit", command: "quit", description: "Exit SquadAI"},
}

func (m Model) viewIntro() string {
	var b strings.Builder

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
	menuContent.WriteString(headingStyle.Render("Main Menu") + "\n\n")

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

func (m Model) viewRunning() string {
	if len(m.applyEvents) > 0 {
		return m.viewApplyProgress()
	}
	return mutedStyle.Render("Running...") + "\n"
}

// viewApplyProgress renders live progress events for the apply command.
func (m Model) viewApplyProgress() string {
	var content strings.Builder

	// Counter header.
	var done int
	for _, ev := range m.applyEvents {
		if ev.Type == pipeline.EventStepDone || ev.Type == pipeline.EventStepSkipped || ev.Type == pipeline.EventStepFailed {
			done++
		}
	}
	total := m.applyTotal
	if total > 0 {
		content.WriteString(headingStyle.Render(fmt.Sprintf("Applying… [%d/%d]", done, total)))
	} else {
		content.WriteString(headingStyle.Render("Applying…"))
	}
	content.WriteString("\n\n")

	// Show last events.
	const maxShow = 8
	events := m.applyEvents
	if len(events) > maxShow {
		events = events[len(events)-maxShow:]
	}
	for _, ev := range events {
		switch ev.Type {
		case pipeline.EventStepStart:
			content.WriteString(mutedStyle.Render("  → " + shortEventLine(ev)))
		case pipeline.EventStepDone:
			content.WriteString(successStyle.Render("  ✓ " + shortEventLine(ev)))
		case pipeline.EventStepSkipped:
			content.WriteString(mutedStyle.Render("  · " + shortEventLine(ev)))
		case pipeline.EventStepFailed:
			content.WriteString(errorStyle.Render("  ✗ " + shortEventLine(ev)))
		default:
			continue
		}
		content.WriteString("\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	return b.String()
}

// shortEventLine returns a compact single-line description for a step event.
func shortEventLine(ev pipeline.Event) string {
	base := string(ev.Component)
	if ev.Adapter != "" {
		base += " · " + ev.Adapter
	}
	if ev.Action != "" && ev.Type == pipeline.EventStepStart {
		base += " · " + ev.Action
	}
	if ev.Type == pipeline.EventStepFailed && ev.Err != nil {
		base += " — " + ev.Err.Error()
	}
	return base
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
