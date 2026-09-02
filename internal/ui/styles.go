package ui

import "github.com/charmbracelet/lipgloss"

// A paleta usa cores adaptativas para que o board fique legível tanto em
// terminal claro quanto escuro.
var (
	colAccent = lipgloss.AdaptiveColor{Light: "#7D56F4", Dark: "#B39DFF"}
	colMuted  = lipgloss.AdaptiveColor{Light: "#6C6C6C", Dark: "#8A8A8A"}
	colFaint  = lipgloss.AdaptiveColor{Light: "#B0B0B0", Dark: "#5A5A5A"}
	colErr    = lipgloss.AdaptiveColor{Light: "#B01030", Dark: "#FF6B8A"}
	colOK     = lipgloss.AdaptiveColor{Light: "#0A7B5A", Dark: "#4ED9A8"}
)

var (
	// Cabeçalho de coluna: a focada ganha cor e sublinhado.
	styleColTitle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colMuted).
			Padding(0, 1)

	styleColTitleFocus = styleColTitle.
				Foreground(colAccent).
				Underline(true)

	styleColCount = lipgloss.NewStyle().Foreground(colFaint)

	// Cards. O selecionado ganha borda de destaque; os demais, borda discreta.
	styleCard = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colFaint).
			Padding(0, 1)

	styleCardSelected = styleCard.
				BorderForeground(colAccent).
				Bold(true)

	styleCardMeta = lipgloss.NewStyle().Foreground(colFaint)

	styleEmpty = lipgloss.NewStyle().
			Foreground(colFaint).
			Italic(true).
			Padding(0, 1)

	styleStatus = lipgloss.NewStyle().Foreground(colMuted).Padding(0, 1)
	styleErrBar = lipgloss.NewStyle().Foreground(colErr).Padding(0, 1)
	styleOKBar  = lipgloss.NewStyle().Foreground(colOK).Padding(0, 1)

	stylePrompt = lipgloss.NewStyle().Foreground(colAccent).Bold(true).Padding(0, 1)

	styleOverlay = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colAccent).
			Padding(1, 2)

	styleHelpKey  = lipgloss.NewStyle().Foreground(colAccent).Bold(true)
	styleHelpDesc = lipgloss.NewStyle().Foreground(colMuted)
)
