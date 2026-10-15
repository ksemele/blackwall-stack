// Package app holds the root tea.Model and view router.
package app

// SwitchViewMsg requests a transition to a top-level view by name.
type SwitchViewMsg struct{ Name string }

// SwitchToDetailMsg is emitted by `:agents` list when the user presses Enter.
type SwitchToDetailMsg struct{ AgentID string }

// BackMsg pops a sub-screen (e.g. agent detail) back to its parent.
type BackMsg struct{}

// ContextChangedMsg announces a context switch initiated from `:contexts`.
type ContextChangedMsg struct{ Name string }

// ContextSwitchedMsg is dispatched when a context switch has been prepared
// off the UI loop (tunnel up, clients built, config saved). On success the app
// runs Apply on the UI loop — so the swap never races with rendering — and
// navigates to agents. Only one switch runs at a time.
type ContextSwitchedMsg struct {
	Name  string
	Err   error
	Apply func()
}

// ShowAgentAlertsMsg asks the app to open `:alerts` filtered to one agent
// (sent by the agent card's `l` key). The app activates the view named
// ":alerts" and passes it this message; the view applies the filter.
type ShowAgentAlertsMsg struct {
	AgentID   string
	AgentName string
}
