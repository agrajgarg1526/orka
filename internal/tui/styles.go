package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorPrimary = lipgloss.Color("#7C3AED")
	colorSuccess = lipgloss.Color("#10B981")
	colorError   = lipgloss.Color("#EF4444")
	colorWarning = lipgloss.Color("#F59E0B")
	colorMuted   = lipgloss.Color("#6B7280")
	colorLive    = lipgloss.Color("#3B82F6")

	StyleTitle = lipgloss.NewStyle().Bold(true).Foreground(colorPrimary)

	StyleColumnHeader = lipgloss.NewStyle().
				Bold(true).
				Padding(0, 1).
				Foreground(lipgloss.Color("#F9FAFB"))

	StyleCard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(0, 1).
			Width(18)

	StyleCardSelected = StyleCard.
				BorderForeground(colorPrimary)

	StyleCardError = StyleCard.
			BorderForeground(colorError)

	StyleCardLive = StyleCard.
			BorderForeground(colorLive)

	StyleStatusLive  = lipgloss.NewStyle().Foreground(colorLive).Bold(true)
	StyleStatusDone  = lipgloss.NewStyle().Foreground(colorSuccess)
	StyleStatusError = lipgloss.NewStyle().Foreground(colorError)
	StyleStatusMuted = lipgloss.NewStyle().Foreground(colorMuted)

	StyleHelp = lipgloss.NewStyle().Foreground(colorMuted).Italic(true)

	StylePaneHeader = lipgloss.NewStyle().
			Bold(true).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorMuted).
			Width(30)

	StyleConfirmPrompt = lipgloss.NewStyle().
				Foreground(colorWarning).
				Bold(true)
)
