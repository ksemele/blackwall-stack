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
