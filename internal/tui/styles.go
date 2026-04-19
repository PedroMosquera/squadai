package tui

import "github.com/charmbracelet/lipgloss"

var (
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true).
			Align(lipgloss.Center)

	headingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Bold(true)

	activeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("12")).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	badgeActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("2"))

	badgeDisabledStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))

	methodologyBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("5")).
				Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))

	authBadgeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")) // yellow — signals attention needed
)
