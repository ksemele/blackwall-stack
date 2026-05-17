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
