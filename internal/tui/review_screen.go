package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/PedroMosquera/squadai/internal/domain"
)

// reviewDecision is the outcome of the pre-apply review screen.
type reviewDecision int

const (
	reviewPending reviewDecision = iota
	reviewConfirmed
	reviewCanceled
)

// reviewPane tracks whether the user is on the entry list or drilled into a
// single entry's diff + conflicts.
type reviewPane int

const (
	reviewPaneList reviewPane = iota
	reviewPaneDetail
)

// reviewModel renders a two-pane pre-apply review over a set of PreviewEntries.
// It never mutates state — its only output is a confirmed/canceled decision
// plus the per-conflict overwrite decisions captured while drilling down.
type reviewModel struct {
	entries     []domain.PreviewEntry
	pane        reviewPane
	cursor      int
	conflictIdx int
	scroll      int
	decision    reviewDecision
	width       int
	height      int

	// overrides records per-conflict user consent to overwrite. Key format:
	// "<targetPath>::<conflictKey>". Absence == keep (the default).
	overrides map[string]bool
}

func newReviewModel(entries []domain.PreviewEntry) reviewModel {
	return reviewModel{
		entries:   entries,
		width:     80,
		height:    24,
		decision:  reviewPending,
		overrides: make(map[string]bool),
	}
}

// overrideKey computes the stable map key for a given (targetPath, conflictKey)
// pair. Format matches what runApplyImpl will feed into ApplyPolicy.Overrides.
func overrideKey(targetPath, conflictKey string) string {
	return targetPath + "::" + conflictKey
}

// currentEntry returns the entry the cursor points at, or the zero value if
// the cursor is out of bounds (defensive — render paths already guard).
func (m reviewModel) currentEntry() domain.PreviewEntry {
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return domain.PreviewEntry{}
	}
	return m.entries[m.cursor]
}

// Policy materializes the user's per-conflict decisions into a domain.ApplyPolicy
// that the caller can hand to the pipeline executor.
func (m reviewModel) Policy() domain.ApplyPolicy {
	out := make(map[string]map[string]bool)
	for _, e := range m.entries {
		for _, c := range e.Conflicts {
			if m.overrides[overrideKey(e.TargetPath, c.Key)] {
				inner, ok := out[e.TargetPath]
				if !ok {
					inner = make(map[string]bool)
					out[e.TargetPath] = inner
				}
				inner[c.Key] = true
			}
		}
	}
	return domain.ApplyPolicy{Overrides: out}
}

func (m reviewModel) Init() tea.Cmd { return nil }

func (m reviewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.pane {
		case reviewPaneList:
			return m.updateList(msg)
		case reviewPaneDetail:
			return m.updateDetail(msg)
		}
	}
	return m, nil
}

func (m reviewModel) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q", "n", "esc":
		m.decision = reviewCanceled
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}
	case "home", "g":
		m.cursor = 0
	case "end", "G":
		m.cursor = len(m.entries) - 1
	case "enter", "right", "l":
		if len(m.entries) > 0 {
			m.pane = reviewPaneDetail
			m.scroll = 0
			m.conflictIdx = 0
		}
	case "y", "a":
		m.decision = reviewConfirmed
		return m, tea.Quit
	}
	return m, nil
}

func (m reviewModel) updateDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	entry := m.currentEntry()
	hasConflicts := len(entry.Conflicts) > 0

	switch msg.String() {
	case "ctrl+c":
		m.decision = reviewCanceled
		return m, tea.Quit
	case "esc", "backspace", "left", "h":
		m.pane = reviewPaneList
	case "tab":
		if hasConflicts {
			m.conflictIdx = (m.conflictIdx + 1) % len(entry.Conflicts)
		}
	case "shift+tab":
		if hasConflicts {
			m.conflictIdx = (m.conflictIdx - 1 + len(entry.Conflicts)) % len(entry.Conflicts)
		}
	case "up":
		if m.scroll > 0 {
			m.scroll--
		}
	case "down":
		m.scroll++
	case "pgup":
		m.scroll -= 10
		if m.scroll < 0 {
			m.scroll = 0
		}
	case "pgdown":
		m.scroll += 10
	case "o":
		if hasConflicts && m.conflictIdx < len(entry.Conflicts) {
			c := entry.Conflicts[m.conflictIdx]
			key := overrideKey(entry.TargetPath, c.Key)
			m.overrides[key] = !m.overrides[key]
			if !m.overrides[key] {
				delete(m.overrides, key)
			}
		}
	case "y", "a":
		m.decision = reviewConfirmed
		return m, tea.Quit
	case "n", "q":
		m.decision = reviewCanceled
		return m, tea.Quit
	}
	return m, nil
}

func (m reviewModel) View() string {
	if m.pane == reviewPaneDetail {
		return m.renderDetail()
	}
	return m.renderList()
}

func (m reviewModel) renderList() string {
	var sb strings.Builder

	title := headingStyle.Render("Review planned changes")
	sb.WriteString(title)
	sb.WriteString("\n")
	sb.WriteString(mutedStyle.Render(fmt.Sprintf("%d entr%s to review", len(m.entries), plural(len(m.entries), "y", "ies"))))
	sb.WriteString("\n\n")

	if len(m.entries) == 0 {
		sb.WriteString(mutedStyle.Render("(nothing to apply)"))
		sb.WriteString("\n\n")
		sb.WriteString(mutedStyle.Render("  y/a: confirm   n/esc/q: cancel"))
		return panelStyle.Render(sb.String())
	}

	for i, e := range m.entries {
		cursor := "  "
		style := lipgloss.NewStyle()
		if i == m.cursor {
			cursor = "▸ "
			style = activeStyle
		}
		marker := actionMarker(e.Action)
		conflicts := ""
		if n := len(e.Conflicts); n > 0 {
			overridden := m.overriddenCount(e)
			if overridden == n {
				conflicts = successStyle.Render(fmt.Sprintf("  [%d conflict%s: all overwrite]", n, plural(n, "", "s")))
			} else if overridden > 0 {
				conflicts = errorStyle.Render(fmt.Sprintf("  [%d conflict%s: %d overwrite]", n, plural(n, "", "s"), overridden))
			} else {
				conflicts = errorStyle.Render(fmt.Sprintf("  [%d conflict%s]", n, plural(n, "", "s")))
			}
		}
		line := fmt.Sprintf("%s%s %s · %s%s", cursor, marker, e.Component, e.TargetPath, conflicts)
		sb.WriteString(style.Render(line))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(mutedStyle.Render("  ↑/↓ move   enter: view diff   y/a: apply all   n/esc: cancel"))

	return panelStyle.Render(sb.String())
}

// overriddenCount counts how many of entry's conflicts the user has toggled
// to overwrite.
func (m reviewModel) overriddenCount(e domain.PreviewEntry) int {
	n := 0
	for _, c := range e.Conflicts {
		if m.overrides[overrideKey(e.TargetPath, c.Key)] {
			n++
		}
	}
	return n
}

func (m reviewModel) renderDetail() string {
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return panelStyle.Render(errorStyle.Render("invalid selection"))
	}
	e := m.entries[m.cursor]

	var sb strings.Builder
	sb.WriteString(headingStyle.Render(fmt.Sprintf("%s  %s", actionMarker(e.Action), e.TargetPath)))
	sb.WriteString("\n")
	sb.WriteString(mutedStyle.Render(fmt.Sprintf("%s · entry %d of %d", e.Component, m.cursor+1, len(m.entries))))
	sb.WriteString("\n\n")

	if len(e.Conflicts) > 0 {
		sb.WriteString(errorStyle.Render(fmt.Sprintf("%d conflict%s — user-owned keys:", len(e.Conflicts), plural(len(e.Conflicts), "", "s"))))
		sb.WriteString("\n")
		for idx, c := range e.Conflicts {
			sb.WriteString("  ")
			if idx == m.conflictIdx {
				sb.WriteString(activeStyle.Render("▸"))
			} else {
				sb.WriteString(" ")
			}
			sb.WriteString(" ")
			badge := mutedStyle.Render("[KEEP]      ")
			if m.overrides[overrideKey(e.TargetPath, c.Key)] {
				badge = successStyle.Render("[OVERWRITE] ")
			}
			sb.WriteString(badge)
			sb.WriteString(activeStyle.Render(c.Key))
			sb.WriteString("\n")
			sb.WriteString("         your value:     ")
			sb.WriteString(c.UserValue)
			sb.WriteString("\n")
			sb.WriteString("         incoming value: ")
			sb.WriteString(c.IncomingValue)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	diff := strings.TrimRight(e.Diff, "\n")
	if diff == "" {
		sb.WriteString(mutedStyle.Render("(no textual diff)"))
		sb.WriteString("\n")
	} else {
		lines := strings.Split(diff, "\n")
		visible := m.detailWindow()
		if m.scroll > len(lines)-1 {
			m.scroll = max(0, len(lines)-1)
		}
		end := m.scroll + visible
		if end > len(lines) {
			end = len(lines)
		}
		for _, line := range lines[m.scroll:end] {
			sb.WriteString(styleDiffLine(line))
			sb.WriteString("\n")
		}
		if len(lines) > visible {
			sb.WriteString(mutedStyle.Render(fmt.Sprintf("  -- lines %d-%d of %d --", m.scroll+1, end, len(lines))))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	if len(e.Conflicts) > 0 {
		sb.WriteString(mutedStyle.Render("  tab: next conflict   o: toggle overwrite   ↑/↓ scroll diff   esc: back   y/a: apply   n: cancel"))
	} else {
		sb.WriteString(mutedStyle.Render("  ↑/↓ scroll   esc/h: back to list   y/a: apply all   n: cancel"))
	}

	return panelStyle.Render(sb.String())
}

// detailWindow returns how many diff lines to show; floor of 8 keeps the view
// usable when the terminal hasn't sent a WindowSizeMsg yet.
func (m reviewModel) detailWindow() int {
	const headerChrome = 10
	window := m.height - headerChrome
	if window < 8 {
		return 8
	}
	return window
}

func styleDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		return mutedStyle.Render(line)
	case strings.HasPrefix(line, "@@"):
		return activeStyle.Render(line)
	case strings.HasPrefix(line, "+"):
		return successStyle.Render(line)
	case strings.HasPrefix(line, "-"):
		return errorStyle.Render(line)
	default:
		return line
	}
}

func actionMarker(a domain.ActionType) string {
	switch a {
	case domain.ActionCreate:
		return successStyle.Render("+ create")
	case domain.ActionUpdate:
		return activeStyle.Render("~ update")
	case domain.ActionDelete:
		return errorStyle.Render("- delete")
	case domain.ActionSkip:
		return mutedStyle.Render("· skip  ")
	default:
		return string(a)
	}
}

func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return singular
	}
	return pluralForm
}
