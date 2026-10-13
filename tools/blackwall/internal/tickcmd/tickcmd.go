// Package tickcmd wraps tea.Tick so views can subscribe to
// periodic ticks without depending on Bubbletea internals.
package tickcmd

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TickMsg is delivered to Update on each interval expiry.
type TickMsg struct{ Time time.Time }

// Every returns a tea.Cmd that fires a single TickMsg after d elapses.
// Callers re-arm the next tick from their Update to avoid drift.
func Every(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg { return TickMsg{Time: t} })
}
