package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/PedroMosquera/squadai/internal/cli"
	"github.com/PedroMosquera/squadai/internal/doctor"
	"github.com/PedroMosquera/squadai/internal/domain"
)

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
			tier := role.Model
			if tier == "" {
				tier = "standard"
			}
			content.WriteString(fmt.Sprintf("  %-14s %s  %s\n",
				name+":", role.Description,
				mutedStyle.Render("· "+tier)))
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

// viewDoctor renders the doctor screen with check results grouped by category.
func (m Model) viewDoctor() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("SquadAI Doctor") + "\n\n")

	if m.doctorResults == nil {
		content.WriteString(mutedStyle.Render("Running checks...") + "\n")
	} else if len(m.doctorResults) == 0 {
		content.WriteString(mutedStyle.Render("No checks returned.") + "\n")
	} else {
		// Group by category.
		catOrder := []string{
			"Environment",
			"AI Agents",
			"Project Configuration",
			"MCP Servers",
			"Filesystem",
			"Config Drift",
		}
		grouped := make(map[string][]doctor.CheckResult)
		for _, r := range m.doctorResults {
			grouped[r.Category] = append(grouped[r.Category], r)
		}

		for _, catName := range catOrder {
			items, ok := grouped[catName]
			if !ok {
				continue
			}
			content.WriteString(headingStyle.Render(catName) + "\n")
			for _, r := range items {
				icon := doctorStatusIcon(r.Status)
				content.WriteString(fmt.Sprintf("  %s  %s\n", icon, r.Message))
			}
			content.WriteString("\n")
		}

		// Summary.
		var pass, warns, fails, skips int
		var hasFixable bool
		for _, r := range m.doctorResults {
			switch r.Status {
			case doctor.CheckPass:
				pass++
			case doctor.CheckWarn:
				warns++
			case doctor.CheckFail:
				fails++
				if r.AutoFixable {
					hasFixable = true
				}
			case doctor.CheckSkip:
				skips++
			}
		}
		summary := fmt.Sprintf("pass: %d  warn: %d  fail: %d  skip: %d", pass, warns, fails, skips)
		if fails > 0 {
			content.WriteString(errorStyle.Render(summary) + "\n")
		} else if warns > 0 {
			content.WriteString(authBadgeStyle.Render(summary) + "\n")
		} else {
			content.WriteString(successStyle.Render(summary) + "\n")
		}

		if m.doctorFixMsg != "" {
			content.WriteString("\n")
			content.WriteString(mutedStyle.Render(m.doctorFixMsg) + "\n")
		} else if hasFixable {
			content.WriteString("\n")
			content.WriteString(mutedStyle.Render("Press f to auto-fix fixable issues.") + "\n")
		}
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("f: auto-fix   esc: back to menu"))
	return b.String()
}

// doctorStatusIcon returns a lipgloss-styled status icon string for a CheckStatus.
func doctorStatusIcon(s doctor.CheckStatus) string {
	switch s {
	case doctor.CheckPass:
		return successStyle.Render("✓")
	case doctor.CheckWarn:
		return authBadgeStyle.Render("⚠")
	case doctor.CheckFail:
		return errorStyle.Render("✗")
	default:
		return mutedStyle.Render("──")
	}
}

// ─── watch screen ────────────────────────────────────────────────────────────

func (m Model) viewWatch() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("Drift Monitor") + "\n\n")

	if len(m.watchResults) == 0 {
		if m.watchChecking {
			content.WriteString(mutedStyle.Render("Checking managed files…") + "\n")
		} else {
			content.WriteString(successStyle.Render("✓ All managed files are intact.") + "\n")
		}
	} else {
		var drifted, intact int
		for _, r := range m.watchResults {
			if r.Drifted() {
				drifted++
			} else {
				intact++
			}
		}

		if drifted > 0 {
			content.WriteString(authBadgeStyle.Render(fmt.Sprintf("⚠ %d file(s) drifted, %d intact", drifted, intact)) + "\n\n")
			for _, r := range m.watchResults {
				if !r.Drifted() {
					continue
				}
				content.WriteString(fmt.Sprintf("  %s  %s\n",
					errorStyle.Render("✗"),
					r.Path))
				content.WriteString(fmt.Sprintf("     %s  %s\n",
					mutedStyle.Render(string(r.Kind)),
					mutedStyle.Render(r.Detail)))
			}
		} else {
			content.WriteString(successStyle.Render(fmt.Sprintf("✓ All %d managed file(s) intact.", intact)) + "\n")
		}
	}

	if !m.watchLastAt.IsZero() {
		ts := m.watchLastAt.Format("15:04:05")
		indicator := mutedStyle.Render("·")
		if m.watchChecking {
			indicator = authBadgeStyle.Render("○")
		}
		content.WriteString("\n" + mutedStyle.Render(fmt.Sprintf("Last check: %s  %s", ts, indicator)) + "\n")
	}
	content.WriteString(mutedStyle.Render("Refreshes every 3 s. Events are written to .squadai/audit.log.") + "\n")

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("esc: back to menu"))
	return b.String()
}

// ─── audit screen ────────────────────────────────────────────────────────────

func (m Model) viewAudit() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("Audit Log") + "\n\n")

	if m.auditErr != nil {
		content.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.auditErr)) + "\n")
	} else if m.auditEvents == nil {
		content.WriteString(mutedStyle.Render("Loading…") + "\n")
	} else if len(m.auditEvents) == 0 {
		content.WriteString(mutedStyle.Render("No events recorded yet.") + "\n")
		content.WriteString(mutedStyle.Render("Run 'squadai watch' or 'squadai apply' to generate events.") + "\n")
	} else {
		visible := m.auditEvents
		if m.auditScroll < len(visible) {
			visible = visible[m.auditScroll:]
		}
		maxRows := 15
		if len(visible) > maxRows {
			visible = visible[:maxRows]
		}

		for _, e := range visible {
			ts := e.Timestamp.Format("01-02 15:04:05")
			kindStr := string(e.Kind)
			var kindStyled string
			switch {
			case strings.HasPrefix(kindStr, "drift:"):
				kindStyled = authBadgeStyle.Render(kindStr)
			case strings.HasPrefix(kindStr, "verify:fail"), strings.HasPrefix(kindStr, "apply:"):
				kindStyled = errorStyle.Render(kindStr)
			default:
				kindStyled = successStyle.Render(kindStr)
			}
			detail := e.Path
			if e.Detail != "" {
				detail += "  " + e.Detail
			}
			content.WriteString(fmt.Sprintf("  %s  %-28s  %s\n",
				mutedStyle.Render(ts), kindStyled, mutedStyle.Render(detail)))
		}

		total := len(m.auditEvents)
		shown := m.auditScroll + len(visible)
		content.WriteString(fmt.Sprintf("\n%s\n",
			mutedStyle.Render(fmt.Sprintf("%d/%d events  (scroll ↑/↓)", shown, total))))
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("↑/↓: scroll   esc: back"))
	return b.String()
}
