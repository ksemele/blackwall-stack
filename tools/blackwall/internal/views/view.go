// Package views defines the View interface implemented by every
// top-level screen (`:contexts`, `:agents`, ...).
package views

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
)

type View interface {
	tea.Model
	Name() string
	Hints() []footer.Hint
}

// KeyCapturer is implemented by views that sometimes need every key — for
// example while the user types into a filter prompt. While CapturesKeys()
// returns true the app hands keys straight to the view and skips its global
// shortcuts (q, Tab, digits, :, ?, p). ctrl+c always quits.
type KeyCapturer interface {
	CapturesKeys() bool
}
