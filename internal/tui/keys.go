package tui

import (
	"bytes"
	"os"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PedroMosquera/squadai/internal/cli"
	"github.com/PedroMosquera/squadai/internal/domain"
)

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
		return m.toMenu()

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
		case "q":
			m.quitting = true
			return m, tea.Quit
		}

	case screenResult:
		// Any key returns to menu.
		m.output = ""
		m.err = nil
		return m.toMenu()

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
			return m.toMenu()
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
			return m.toMenu()
		}

	case screenDoctor:
		switch key {
		case "f":
			if len(m.doctorResults) > 0 {
				return m, m.runDoctorFixCmd()
			}
		case "esc", "q":
			m.doctorResults = nil
			m.doctorFixMsg = ""
			return m.toMenu()
		}

	case screenWatch:
		switch key {
		case "esc", "q":
			return m.toMenu()
		}

	case screenAudit:
		switch key {
		case "up", "k":
			if m.auditScroll > 0 {
				m.auditScroll--
			}
		case "down", "j":
			maxScroll := len(m.auditEvents) - 10
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.auditScroll < maxScroll {
				m.auditScroll++
			}
		case "esc", "q":
			return m.toMenu()
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
				// Non-custom: skip plugins, go directly to project memory.
				if _, err := os.Stat("docs/memory"); err == nil {
					m.projectMemoryPathExists = true
				} else {
					m.projectMemoryPathExists = false
				}
				m.screen = screenInitProjectMemory
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
			// Check if docs/memory/ already exists before showing the scaffold prompt.
			if _, err := os.Stat("docs/memory"); err == nil {
				m.projectMemoryPathExists = true
			} else {
				m.projectMemoryPathExists = false
			}
			m.screen = screenInitProjectMemory
			return m, nil
		case "esc":
			m.initCursor = 0
			m.screen = screenInitMCP
			return m, nil
		}

	case screenInitProjectMemory:
		switch key {
		case " ":
			m.projectMemoryEnabled = !m.projectMemoryEnabled
		case "enter":
			if m.projectMemoryEnabled && !m.projectMemoryPathExists {
				m.screen = screenInitProjectMemoryScaffold
			} else {
				m.screen = screenInitInstallSummary
			}
			return m, nil
		case "esc":
			m.initCursor = 0
			if m.setupPreset == domain.PresetCustom {
				m.screen = screenInitPlugins
			} else {
				m.screen = screenInitMCP
			}
			return m, nil
		}

	case screenInitProjectMemoryScaffold:
		switch key {
		case "y", "Y":
			m.projectMemoryScaffold = true
			m.screen = screenInitInstallSummary
			return m, nil
		case "n", "N":
			m.projectMemoryScaffold = false
			m.screen = screenInitInstallSummary
			return m, nil
		case "esc":
			m.screen = screenInitProjectMemory
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
		// All canonical agents in fixed order.
		allAgents := []domain.AgentID{
			domain.AgentOpenCode,
			domain.AgentClaudeCode,
			domain.AgentVSCodeCopilot,
			domain.AgentCursor,
			domain.AgentWindsurf,
			domain.AgentPi,
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
					string(domain.AgentPi),
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
			// Pass permissions flag.
			if !m.permissionsEnabled {
				args = append(args, "--no-permissions")
			}
			// Pass memory flag.
			if !m.projectMemoryEnabled {
				args = append(args, "--without-memory")
			}
			return m, func() tea.Msg {
				var buf bytes.Buffer
				err := cli.RunInit(args, &buf)
				return commandResult{output: buf.String(), err: err}
			}
		case "esc":
			m.screen = screenInitProjectMemory
			return m, nil
		}

	case screenInitApplyPrompt:
		switch key {
		case "y", "enter":
			m.screen = screenRunning
			m.output = ""
			m.err = nil
			return m, m.runApplyWithProgress()
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
		case "enter":
			if len(m.skillCat.Categories) == 0 || m.skillCatCursor >= len(m.skillCat.Categories) {
				return m, nil
			}
			skills := m.skillCat.Categories[m.skillCatCursor].Skills
			if m.skillScrollIndex >= len(skills) {
				return m, nil
			}
			selected := skills[m.skillScrollIndex]
			if selected.Install == "" {
				return m, nil
			}
			installBase := strings.TrimSpace(m.skillCat.InstallCommand)
			if installBase == "" {
				installBase = "npx skills add -y"
			}
			m.pendingSkillName = selected.Name
			m.pendingSkillCmd = installBase + " " + selected.Install
			m.screen = screenSkillInstallConfirm
			return m, nil
		case "esc", "q":
			return m.toMenu()
		}

	case screenSkillInstallConfirm:
		switch key {
		case "y", "Y", "enter":
			cmdStr := m.pendingSkillCmd
			m.screen = screenRunning
			m.output = ""
			m.err = nil
			return m, func() tea.Msg {
				return runShellCommand(cmdStr)
			}
		case "n", "N", "esc", "q":
			m.pendingSkillName = ""
			m.pendingSkillCmd = ""
			m.screen = screenSkillBrowser
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
			return m.toMenu()
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
