package agents

import tea "github.com/charmbracelet/bubbletea"

// subView is an in-package interface for tab-scoped sub-models.
type subView interface {
	Init() tea.Cmd
	Update(tea.Msg) (subView, tea.Cmd)
	View() string
}

// keyCapturer is implemented by sub-views that want to consume all key events
// while in some modal state (e.g. typing into a filter prompt). When the active
// sub-view's CapturesKeys() returns true, DetailModel bypasses its own KeyMsg
// handling (tab hotkeys, esc/back, r/refresh) and forwards keys directly.
type keyCapturer interface {
	CapturesKeys() bool
}
