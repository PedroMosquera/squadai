package tui

import (
	"bytes"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/PedroMosquera/squadai/internal/assets"
	"github.com/PedroMosquera/squadai/internal/cli"
	"github.com/PedroMosquera/squadai/internal/domain"
)

// quickInitDoneMsg carries the result of the quick track's init run. It is a
// distinct message type from commandResult so the init→apply chain cannot be
// confused with a plain command finishing.
type quickInitDoneMsg struct {
	output string
	err    error
}

// quickStyleOption is one answer to "How do you like to work?".
type quickStyleOption struct {
	label  string
	desc   string
	preset domain.SetupPreset
}

// quickStyleOptions maps plain-language working styles to canonical CLI
// presets — the preset names stay the single vocabulary.
var quickStyleOptions = []quickStyleOption{
	{
		label:  "Keep it simple",
		desc:   "A light setup: sensible rules, low cost, no extra process.",
		preset: domain.PresetSoloMinimal,
	},
	{
		label:  "Balanced everyday setup   (recommended)",
		desc:   "Your AI tools write tests as they go and use a smart mix of fast and powerful models.",
		preset: domain.PresetSoloPower,
	},
	{
		label:  "Full team of specialists",
		desc:   "A squad of AI roles (planner, builder, reviewer…) that work from written specs. Most thorough, uses more tokens.",
		preset: domain.PresetFullSquad,
	},
}

// quickExtraOption is one optional extra on the quick track. Extras map to
// either the project-memory component or a curated MCP server. The community
// knowledge-graph server ("memory") is intentionally NOT offered here — it
// overlaps with Project Memory and stays an advanced-track choice.
type quickExtraOption struct {
	key        string // "project-memory" or a curated MCP server name
	label      string
	desc       string
	isMemory   bool // true for the project-memory component toggle
	preChecked bool
}

var quickExtraOptions = []quickExtraOption{
	{
		key:        "project-memory",
		label:      "Project memory",
		desc:       "A shared project notebook your AI tools read and keep up to date.",
		isMemory:   true,
		preChecked: true,
	},
	{
		key:        "context7",
		label:      "Live documentation lookup",
		desc:       "Lets your tools read up-to-date docs for the libraries you use.",
		preChecked: true,
	},
	{
		key:   "github",
		label: "GitHub access",
		desc:  "Lets your tools read issues and pull requests on GitHub.",
	},
	{
		key:   "sentry",
		label: "Error tracking (Sentry)",
		desc:  "Lets your tools look up production errors and stack traces.",
	},
	{
		key:   "sequential-thinking",
		label: "Step-by-step reasoning helper",
		desc:  "Helps your tools think through tricky problems in explicit steps.",
	},
}

// startQuickSetup resets the model into the quick setup track. merge controls
// whether the eventual init run merges with an existing config (menu re-run)
// or writes a fresh one (first run).
func (m Model) startQuickSetup(merge bool) Model {
	m.screen = screenQuickAgents
	m.initCursor = 0
	m.quickMerge = merge
	m.quickFlow = false
	m.setupPreset = ""
	m.methodology = ""
	m.modelTier = ""
	m.projectMemoryEnabled = true
	m.projectMemoryScaffold = true // quick track never prompts; scaffold when missing
	m.agentSelections = make(map[string]bool)
	for _, a := range m.adapters {
		m.agentSelections[string(a.ID())] = true
	}
	m.quickExtras = make(map[string]bool, len(quickExtraOptions))
	for _, e := range quickExtraOptions {
		m.quickExtras[e.key] = e.preChecked
	}
	return m
}

// ─── key handling ────────────────────────────────────────────────────────────

func (m Model) handleQuickAgentsKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.initCursor > 0 {
			m.initCursor--
		}
	case "down", "j":
		if m.initCursor < len(allCanonicalAgents)-1 {
			m.initCursor++
		}
	case " ":
		if m.initCursor < len(allCanonicalAgents) {
			id := string(allCanonicalAgents[m.initCursor])
			if m.agentSelections == nil {
				m.agentSelections = make(map[string]bool)
			}
			m.agentSelections[id] = !m.agentSelections[id]
		}
	case "enter":
		if countSelected(m.agentSelections) == 0 {
			// Guard: at least one tool must be selected to continue.
			return m, nil
		}
		m.initCursor = 1 // default to the recommended balanced style
		m.screen = screenQuickStyle
		return m, nil
	case "m":
		return m.toMenu()
	}
	return m, nil
}

func (m Model) handleQuickStyleKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.initCursor > 0 {
			m.initCursor--
		}
	case "down", "j":
		if m.initCursor < len(quickStyleOptions)-1 {
			m.initCursor++
		}
	case "enter":
		m.setupPreset = quickStyleOptions[m.initCursor].preset
		m.initCursor = 0
		m.screen = screenQuickExtras
		return m, nil
	case "esc":
		m.initCursor = 0
		m.screen = screenQuickAgents
		return m, nil
	}
	return m, nil
}

func (m Model) handleQuickExtrasKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "up", "k":
		if m.initCursor > 0 {
			m.initCursor--
		}
	case "down", "j":
		if m.initCursor < len(quickExtraOptions)-1 {
			m.initCursor++
		}
	case " ":
		if m.initCursor < len(quickExtraOptions) {
			if m.quickExtras == nil {
				m.quickExtras = make(map[string]bool)
			}
			k := quickExtraOptions[m.initCursor].key
			m.quickExtras[k] = !m.quickExtras[k]
		}
	case "enter":
		m.initCursor = 0
		m.screen = screenQuickSummary
		return m, nil
	case "esc":
		m.initCursor = quickStyleIndex(m.setupPreset)
		m.screen = screenQuickStyle
		return m, nil
	}
	return m, nil
}

func (m Model) handleQuickSummaryKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "enter":
		if countSelected(m.agentSelections) == 0 {
			m.initCursor = 0
			m.screen = screenQuickAgents
			return m, nil
		}
		m = m.applyQuickSelections()
		m.screen = screenRunning
		m.output = ""
		m.err = nil
		m.quickFlow = true
		args := m.buildInitArgs()
		return m, func() tea.Msg {
			var buf bytes.Buffer
			err := cli.RunInit(args, &buf)
			return quickInitDoneMsg{output: buf.String(), err: err}
		}
	case "c":
		// Switch to the advanced track, preserving agent selections.
		m = m.applyQuickSelections()
		m.setupPreset = domain.PresetCustom
		m.initCursor = 0
		m.screen = screenInitAdapters
		return m, nil
	case "esc":
		m.initCursor = 0
		m.screen = screenQuickExtras
		return m, nil
	}
	return m, nil
}

// applyQuickSelections translates quick-track answers into the shared wizard
// state consumed by buildInitArgs and the post-install panels.
func (m Model) applyQuickSelections() Model {
	// Extras → MCP selections. Non-nil map: unchecked extras mean "no MCP".
	m.mcpSelections = make(map[string]bool)
	for _, e := range quickExtraOptions {
		if e.isMemory {
			continue
		}
		m.mcpSelections[e.key] = m.quickExtras[e.key]
	}
	m.projectMemoryEnabled = m.quickExtras["project-memory"]
	// Claude Code starts with the SquadAI coordinator only when it is
	// actually one of the selected tools.
	m.setClaudeDefaultAgent = m.agentSelections != nil && m.agentSelections[string(domain.AgentClaudeCode)]
	return m
}

// quickStyleIndex returns the quickStyleOptions index for a preset (default:
// the recommended balanced option).
func quickStyleIndex(p domain.SetupPreset) int {
	for i, o := range quickStyleOptions {
		if o.preset == p {
			return i
		}
	}
	return 1
}

// countSelected counts true values in a selection map.
func countSelected(sel map[string]bool) int {
	n := 0
	for _, v := range sel {
		if v {
			n++
		}
	}
	return n
}

// ─── views ───────────────────────────────────────────────────────────────────

// quickBanner returns the full ASCII wordmark when the panel is wide enough,
// otherwise the compact monogram used in the header.
func (m Model) quickBanner() string {
	if m.panelWidth() >= 50 {
		if banner, err := assets.Read("brand/banner-squadai.txt"); err == nil {
			return bannerStyle.Render(banner)
		}
	}
	return logoStyle.Render("▞▀▚\n▚▄▞")
}

// viewQuickAgents renders Q1: which AI coding tools do you use.
func (m Model) viewQuickAgents() string {
	var content strings.Builder
	content.WriteString(m.quickBanner())
	content.WriteString("\n\n")
	content.WriteString(headingStyle.Render("Welcome to SquadAI") + "\n\n")
	content.WriteString("SquadAI writes one shared setup for all your AI coding tools, so they\n")
	content.WriteString("follow the same rules and remember the same things.\n\n")
	content.WriteString(headingStyle.Render("Which AI coding tools do you use?") + "\n")
	content.WriteString(mutedStyle.Render("We found these on your machine (pre-checked). Toggle any others you use.") + "\n\n")

	detectedSet := make(map[string]bool, len(m.adapters))
	for _, a := range m.adapters {
		detectedSet[string(a.ID())] = true
	}

	for i, agentID := range allCanonicalAgents {
		id := string(agentID)
		checked := m.agentSelections != nil && m.agentSelections[id]

		var checkStr string
		if checked {
			checkStr = badgeActiveStyle.Render("[x]")
		} else {
			checkStr = badgeDisabledStyle.Render("[ ]")
		}

		name := agentDisplayName(agentID)
		var nameStr string
		if i == m.initCursor {
			nameStr = activeStyle.Render(name)
		} else {
			nameStr = name
		}

		var suffix string
		if !detectedSet[id] {
			suffix = mutedStyle.Render("(not found on this machine)")
		}

		content.WriteString(fmt.Sprintf("  %s %-20s %s\n", checkStr, nameStr, suffix))
	}

	if countSelected(m.agentSelections) == 0 {
		content.WriteString("\n" + authBadgeStyle.Render("Select at least one tool to continue.") + "\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("space: toggle   enter: continue   m: full menu   ctrl+c: quit"))
	return b.String()
}

// viewQuickStyle renders Q2: how do you like to work.
func (m Model) viewQuickStyle() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("How do you like to work?") + "\n\n")

	for i, opt := range quickStyleOptions {
		if i == m.initCursor {
			content.WriteString(activeStyle.Render("> "+opt.label) + "\n")
		} else {
			content.WriteString("  " + opt.label + "\n")
		}
		content.WriteString(mutedStyle.Render("    "+opt.desc) + "\n\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("↑/↓: navigate   enter: continue   esc: back"))
	return b.String()
}

// viewQuickExtras renders Q3: optional extras.
func (m Model) viewQuickExtras() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("Anything extra?") + "\n")
	content.WriteString(mutedStyle.Render(`All optional. You can change these later with "Quick Setup".`) + "\n\n")

	for i, opt := range quickExtraOptions {
		var checkStr string
		if m.quickExtras[opt.key] {
			checkStr = badgeActiveStyle.Render("[x]")
		} else {
			checkStr = badgeDisabledStyle.Render("[ ]")
		}

		var labelStr string
		if i == m.initCursor {
			labelStr = activeStyle.Render(opt.label)
		} else {
			labelStr = opt.label
		}

		content.WriteString(fmt.Sprintf("  %s %s\n", checkStr, labelStr))
		content.WriteString(mutedStyle.Render("      "+opt.desc) + "\n\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("space: toggle   enter: continue   esc: back"))
	return b.String()
}

// viewQuickSummary renders Q4: ready to set up.
func (m Model) viewQuickSummary() string {
	var content strings.Builder
	content.WriteString(headingStyle.Render("Ready to set up") + "\n\n")

	// Tools line.
	var tools []string
	for _, agentID := range allCanonicalAgents {
		if m.agentSelections != nil && m.agentSelections[string(agentID)] {
			tools = append(tools, agentDisplayName(agentID))
		}
	}
	toolsLine := "(none selected)"
	if len(tools) > 0 {
		toolsLine = strings.Join(tools, ", ")
	}
	content.WriteString(fmt.Sprintf("  %-8s %s\n", "Tools", toolsLine))

	// Style line.
	style := quickStyleOptions[quickStyleIndex(m.setupPreset)]
	content.WriteString(fmt.Sprintf("  %-8s %s\n", "Style", strings.TrimSpace(strings.Split(style.label, "  ")[0])))

	// Extras line.
	var extras []string
	for _, opt := range quickExtraOptions {
		if m.quickExtras[opt.key] {
			extras = append(extras, opt.label)
		}
	}
	extrasLine := "none"
	if len(extras) > 0 {
		extrasLine = strings.Join(extras, ", ")
	}
	content.WriteString(fmt.Sprintf("  %-8s %s\n", "Extras", extrasLine))

	if m.agentSelections != nil && m.agentSelections[string(domain.AgentClaudeCode)] {
		content.WriteString("\n")
		content.WriteString("Claude Code will start with the SquadAI coordinator by default.\n")
	}

	content.WriteString("\n")
	content.WriteString(mutedStyle.Render(`This writes config files into this project. A backup is taken first, and "Remove SquadAI Config" undoes everything.`) + "\n")

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("enter: set everything up   c: customize (advanced)   esc: back"))
	return b.String()
}

// viewQuickDone renders the post-apply success screen for the quick track.
func (m Model) viewQuickDone() string {
	var content strings.Builder
	content.WriteString(successStyle.Render("✓ You're all set. Open your AI tool in this folder and start working.") + "\n")

	// Show post-install auth guidance when any selected MCPs require credentials.
	if panel := renderPostInstallAuthPanel(m.mcpSelections); panel != "" {
		content.WriteString("\n")
		content.WriteString(panel)
		content.WriteString("\n")
	}

	var b strings.Builder
	b.WriteString(m.renderPanel(strings.TrimRight(content.String(), "\n")))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("Press any key to go to the menu."))
	return b.String()
}
