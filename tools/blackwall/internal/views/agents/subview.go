package agents

import tea "github.com/charmbracelet/bubbletea"

// subView is an in-package interface for tab-scoped sub-models.
type subView interface {
	Init() tea.Cmd
	Update(tea.Msg) (subView, tea.Cmd)
	View() string
}
