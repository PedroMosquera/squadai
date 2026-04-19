package tui

import "github.com/charmbracelet/lipgloss"

var (
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	// headerPanelStyle is the persistent wizard header — dark-blue rounded border.
	headerPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("33")).
				Padding(0, 2)

	// logoStyle renders the small ASCII monogram on the left of the header.
	logoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33")).
			Bold(true).
			MarginRight(2)

	// headerTitleStyle renders "SquadAI vX.Y.Z" on the right of the header.
	headerTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("12")).
				Bold(true)

	// headerTaglineStyle renders the subtitle below the title.
	headerTaglineStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	headingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("39")).
			Bold(true)

	activeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	// badgeActiveStyle marks an enabled adapter / item — cyan to match the blue palette.
	badgeActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("14"))

	badgeDisabledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	methodologyBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("5")).
				Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)

	// successStyle marks ✓ / pass states — cyan keeps the palette cohesive
	// while still reading as positive (red/yellow remain reserved for error/warn).
	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("14"))

	authBadgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")) // yellow — signals attention needed
)
