package tui

import (
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PedroMosquera/squadai/internal/doctor"
	"github.com/PedroMosquera/squadai/internal/domain"
	"github.com/PedroMosquera/squadai/internal/governance"
	"github.com/PedroMosquera/squadai/internal/pipeline"
)

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
	screenInitProjectMemory         // project-memory toggle
	screenInitProjectMemoryScaffold // conditional: prompt to scaffold when path missing
	screenInitModelTier
	screenInitAdapters       // agent selection checkboxes
	screenInitPreset         // setup preset radio (full-squad / lean / custom)
	screenInitInstallSummary // review and confirm before applying
	screenSkillBrowser
	screenSkillInstallConfirm // "Install skill X?" Y/N confirmation
	screenInitApplyPrompt     // "Apply now?" prompt after successful init
	screenRemoveConfirm       // "Remove SquadAI config" confirmation screen
	screenClaudeDefaultAgent  // "Set orchestrator as default Claude agent?" yes/no
	screenDoctor              // pre-flight diagnostics screen
	screenWatch               // live drift monitor
	screenAudit               // governance audit log
)

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
	methodology        domain.Methodology
	initCursor         int
	mcpSelections      map[string]bool
	pluginSelections   map[string]bool
	permissionsEnabled bool // security overlay: deny .env reads, confirm dangerous commands
	modelTier          domain.ModelTier

	// Agent selection and preset (new screens).
	agentSelections map[string]bool    // key=AgentID string, val=selected
	setupPreset     domain.SetupPreset // "full-squad", "lean", "custom"

	// Project memory wizard state.
	projectMemoryEnabled    bool // whether to enable the memory scaffold
	projectMemoryScaffold   bool // whether to scaffold the memory folder
	projectMemoryPathExists bool // whether docs/memory/ already exists

	// Skill browser state.
	skillCat         skillCatalog
	skillCatErr      error
	skillCatCursor   int // selected category index
	skillScrollIndex int // first visible skill within the current category

	// Skill install confirmation state.
	pendingSkillName string // skill name awaiting install confirmation
	pendingSkillCmd  string // full shell command that will be executed on confirm

	// Init apply-prompt state.
	initJustCompleted bool   // set before launching init; triggers apply-prompt on success
	initOutput        string // stores init output while on apply-prompt screen

	// Claude default agent prompt state.
	setClaudeDefaultAgent bool // whether to write .claude/settings.json "agent" field

	// Remove confirmation state.
	removePreview string // dry-run output shown on confirmation screen

	// Doctor screen state.
	doctorResults []doctor.CheckResult
	doctorFixMsg  string // message shown after auto-fix attempt

	// Apply-with-progress state.
	applyProgressCh <-chan pipeline.Event // receives events while apply runs
	applyEvents     []pipeline.Event      // accumulated recent events
	applyTotal      int                   // total steps, set on EventPipelineStart

	// Drift badge shown on the menu screen.
	driftBadgeReady bool
	driftBadgeCount int // number of drifted files; 0 + ready = clean

	// Watch screen state.
	watchResults  []governance.DriftResult
	watchChecking bool
	watchLastAt   time.Time

	// Audit screen state.
	auditEvents []governance.Event
	auditErr    error
	auditScroll int
}

// NewModel creates a TUI model with the given state.
func NewModel(version string, mode domain.OperationalMode, adapters []domain.Adapter, homeDir string) Model {
	return Model{
		version:             version,
		mode:                mode,
		adapters:            adapters,
		homeDir:             homeDir,
		screen:              screenIntro,
		modelTier:           domain.ModelTierBalanced,
		permissionsEnabled:  true, // security overlay is on by default
		projectMemoryEnabled: true, // memory scaffold is on by default
	}
}

// toMenu transitions the model to the main menu and fires a background drift
// check so the badge is always fresh when the user lands on the menu.
func (m Model) toMenu() (Model, tea.Cmd) {
	m.screen = screenMenu
	m.driftBadgeReady = false
	return m, m.runDriftBadgeCmd()
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
	case doctorCheckResult:
		m.doctorResults = msg.results
		// Stay on screenDoctor (was already set when cmd was launched).
		return m, nil
	case doctorFixDone:
		m.doctorFixMsg = msg.msg
		return m, nil
	case driftBadgeMsg:
		m.driftBadgeReady = true
		m.driftBadgeCount = msg.count
		return m, nil
	case watchDriftMsg:
		if m.screen == screenWatch {
			m.watchResults = msg.results
			m.watchChecking = false
			m.watchLastAt = msg.at
			return m, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return watchTickMsg{} })
		}
		return m, nil
	case watchTickMsg:
		if m.screen == screenWatch {
			m.watchChecking = true
			return m, m.runWatchDriftCmd()
		}
		return m, nil
	case auditLoadedMsg:
		m.auditEvents = msg.events
		m.auditErr = msg.err
		m.auditScroll = 0
		return m, nil
	case pipelineEventMsg:
		ev := msg.event
		if ev.Type == pipeline.EventPipelineStart {
			m.applyTotal = ev.Total
		}
		// Keep a rolling window of the last 8 events for display.
		m.applyEvents = append(m.applyEvents, ev)
		if len(m.applyEvents) > 8 {
			m.applyEvents = m.applyEvents[len(m.applyEvents)-8:]
		}
		// Schedule the next read from the channel.
		return m, listenForPipelineEvent(m.applyProgressCh)
	}
	return m, nil
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

// renderHeader renders the persistent wizard header — small ASCII logo on the
// left, product title + tagline on the right, dark-blue rounded border.
// Shown on every screen for consistent branding.
func (m Model) renderHeader() string {
	logo := logoStyle.Render("▞▀▚\n▚▄▞")
	title := headerTitleStyle.Render("SquadAI " + formatVersion(m.version))
	tagline := headerTaglineStyle.Render("One config. Every AI agent. Zero drift.")
	right := lipgloss.JoinVertical(lipgloss.Left, title, tagline)
	body := lipgloss.JoinHorizontal(lipgloss.Center, logo, right)
	return headerPanelStyle.Width(m.panelWidth()).Render(body)
}

// formatVersion normalizes the version string for display: tagged releases
// (e.g. "0.1.1") get a "v" prefix, dev/snapshot builds are shown verbatim.
func formatVersion(v string) string {
	if v == "" || v == "dev" {
		return "dev"
	}
	if v[0] >= '0' && v[0] <= '9' {
		return "v" + v
	}
	return v
}

// View renders the TUI.
func (m Model) View() string {
	if m.quitting {
		return ""
	}

	body := m.viewBody()
	if body == "" {
		return ""
	}
	return m.renderHeader() + "\n\n" + body
}

// viewBody dispatches to the screen-specific view function.
func (m Model) viewBody() string {
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
	case screenInitProjectMemory:
		return m.viewInitProjectMemory()
	case screenInitProjectMemoryScaffold:
		return m.viewInitProjectMemoryScaffold()
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
	case screenSkillInstallConfirm:
		return m.viewSkillInstallConfirm()
	case screenInitApplyPrompt:
		return m.viewInitApplyPrompt()
	case screenRemoveConfirm:
		return m.viewRemoveConfirm()
	case screenClaudeDefaultAgent:
		return m.viewClaudeDefaultAgent()
	case screenDoctor:
		return m.viewDoctor()
	case screenWatch:
		return m.viewWatch()
	case screenAudit:
		return m.viewAudit()
	}
	return ""
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
	case domain.AgentPi:
		return "Pi"
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
