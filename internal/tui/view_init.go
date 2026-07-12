package tui

import (
	"fmt"
	"strings"

	"github.com/PedroMosquera/squadai/internal/cli"
	"github.com/PedroMosquera/squadai/internal/domain"
)

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
	// All canonical agents in fixed order.
	allAgents := []domain.AgentID{
		domain.AgentOpenCode,
		domain.AgentClaudeCode,
		domain.AgentVSCodeCopilot,
		domain.AgentCursor,
		domain.AgentWindsurf,
		domain.AgentPi,
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
	permStr := "enabled"
	if !m.permissionsEnabled {
		permStr = "disabled"
	}
	content.WriteString(fmt.Sprintf("  %-16s %s\n", "Permissions", permStr))

	content.WriteString("\n")
	content.WriteString(headingStyle.Render("Agents") + "\n")

	// List all canonical agents; show only selected ones.
	allAgents := []domain.AgentID{
		domain.AgentOpenCode,
		domain.AgentClaudeCode,
		domain.AgentVSCodeCopilot,
		domain.AgentCursor,
		domain.AgentWindsurf,
		domain.AgentPi,
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
