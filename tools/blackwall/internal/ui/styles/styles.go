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
)
