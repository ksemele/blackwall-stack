// Package styles holds the blackwall dark theme.
package styles

import "github.com/charmbracelet/lipgloss"

var (
	Active       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("10"))
	Disconnected = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	Pending      = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	Dim          = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	HeaderText = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
	StatusBar  = lipgloss.NewStyle().Foreground(lipgloss.Color("15"))
	Footer     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	TabActive   = lipgloss.NewStyle().Bold(true).Underline(true).Foreground(lipgloss.Color("14"))
	TabInactive = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	Error = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)

	// SelectedRow is a full-row k9s-style highlight: cyan background, black foreground.
	SelectedRow = lipgloss.NewStyle().Bold(true).
			Background(lipgloss.Color("14")).
			Foreground(lipgloss.Color("0"))

	// ContextBadge renders the active context name as a yellow badge (k9s-style).
	ContextBadge = lipgloss.NewStyle().
			Background(lipgloss.Color("220")).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1).
			Bold(true)

	// RowDown renders a full table row in red to signal an offline/broken resource (disconnected).
	RowDown = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	// RowPending renders a full table row in yellow for transient/recoverable states.
	RowPending = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))

	// RowGhost renders a full table row in dim grey for agents that were registered
	// but have never connected (never_connected status).
	RowGhost = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	// HRule is the style for the thin horizontal rule that separates the header
	// chrome from the view body.  Color "240" is a mid-dark grey on 256-colour
	// terminals (falls back to colour "8" on 8-colour terminals).
	HRule = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// HelpHeading renders section headings inside the help view.
	HelpHeading = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("14"))
)
