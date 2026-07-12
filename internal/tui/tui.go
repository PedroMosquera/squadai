package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/cli"
	"github.com/PedroMosquera/squadai/internal/doctor"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// skillEntry is a single skill in the curated catalog.
type skillEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Install     string `json:"install"` // owner/repo@skill identifier passed to `npx skills add`
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
		installCmd = "npx skills add -y"
	}
	browseURL := m.skillCat.BrowseURL
	if browseURL == "" {
		browseURL = "https://skills.sh"
	}
	selectedInstall := ""
	if len(currentCat.Skills) > 0 && m.skillScrollIndex < len(currentCat.Skills) {
		selectedInstall = " " + currentCat.Skills[m.skillScrollIndex].Install
	}
	content.WriteString(mutedStyle.Render(
		"Install: "+installCmd+selectedInstall+"  |  Browse more: "+browseURL,
	) + "\n")

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("tab/←/→: category  ↑/↓: skill  enter: install  esc/q: back"))
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

// viewSkillInstallConfirm renders the install confirmation prompt for a
// community skill selected in the browser.
func (m Model) viewSkillInstallConfirm() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("Install community skill") + "\n\n")
	content.WriteString("Skill:   " + activeStyle.Render(m.pendingSkillName) + "\n")
	content.WriteString("Command: " + mutedStyle.Render(m.pendingSkillCmd) + "\n\n")

	if _, err := exec.LookPath("npx"); err != nil {
		content.WriteString(errorStyle.Render("npx not found on PATH.") + "\n")
		content.WriteString(mutedStyle.Render("Install Node.js (https://nodejs.org) and try again.") + "\n\n")
		content.WriteString(mutedStyle.Render("esc: back"))
	} else {
		content.WriteString(mutedStyle.Render("This will run the command above in the current directory.") + "\n\n")
		content.WriteString(mutedStyle.Render("y: install   n/esc: cancel"))
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	return b.String()
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
