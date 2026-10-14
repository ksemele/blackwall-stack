// Package help implements the `:help` view — a single-screen keyboard shortcut reference.
package help

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
)

// View is the help screen.  It is pushed onto the view stack when the user
// presses `?` and popped when they press `q` or `ESC`.
type View struct{}

// New returns a new help View.
func New() *View { return &View{} }

func (v *View) Name() string { return ":help" }

func (v *View) Hints() []footer.Hint {
	return []footer.Hint{
		{Key: "ESC", Desc: "back"},
		{Key: "q", Desc: "quit"},
	}
}

func (v *View) Init() tea.Cmd { return nil }

func (v *View) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			return v, func() tea.Msg { return app.BackMsg{} }
		case "q":
			return v, tea.Quit
		}
	}
	return v, nil
}

func (v *View) View() string {
	var b strings.Builder

	b.WriteString(styles.HelpHeading.Render("Help") + "\n\n")

	b.WriteString(styles.HelpHeading.Render("Global") + "\n")
	b.WriteString(row("?", "Show this help"))
	b.WriteString(row("q", "Quit"))
	b.WriteString(row("1", "Contexts view"))
	b.WriteString(row("2", "Agents view"))
	b.WriteString(row("tab / shift+tab", "Cycle views"))
	b.WriteString(row("ESC", "Back"))
	b.WriteString(row(":", "Open command palette"))
	b.WriteString("\n")

	b.WriteString(styles.HelpHeading.Render("Agents") + "\n")
	b.WriteString(row("↑ / ↓  (or k/j)", "Navigate"))
	b.WriteString(row("↵", "Details"))
	b.WriteString(row("p", "Pause / resume refresh"))
	b.WriteString(row("ctrl+r", "Force refresh"))
	b.WriteString(row("/", "Filter  (not yet implemented)"))
	b.WriteString("\n")

	b.WriteString(styles.HelpHeading.Render("Agent detail") + "\n")
	b.WriteString(row("← / →", "Previous / next tab (← on Overview: back)"))
	b.WriteString(row("ESC", "Back to list"))
	b.WriteString(row("r", "Refresh"))
	b.WriteString("\n")

	b.WriteString(styles.HelpHeading.Render("Contexts") + "\n")
	b.WriteString(row("↑ / ↓  (or k/j)", "Navigate"))
	b.WriteString(row("↵", "Switch context"))
	b.WriteString(row("ctrl+r", "Re-check availability"))
	b.WriteString(row("ESC", "Back"))

	return b.String()
}

// row formats a single help entry as "  key            description\n".
func row(key, desc string) string {
	return fmt.Sprintf("  %-22s  %s\n", key, desc)
}
