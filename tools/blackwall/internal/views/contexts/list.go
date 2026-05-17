// Package contexts implements the `:contexts` k9s-style picker.
package contexts

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
)

type Model struct {
	cfg    config.Config
	order  []string
	cursor int
}

func New(cfg config.Config) *Model {
	names := make([]string, 0, len(cfg.Contexts))
	for n := range cfg.Contexts {
		names = append(names, n)
	}
	sort.Strings(names)
	return &Model{cfg: cfg, order: names}
}

func (m *Model) Name() string { return ":contexts" }

func (m *Model) Hints() []footer.Hint {
	return []footer.Hint{
		{Key: "↑↓", Desc: "nav"},
		{Key: "↵", Desc: "open"},
		{Key: "c", Desc: "set current"},
		{Key: "/", Desc: "filter"},
	}
}

func (m *Model) Init() tea.Cmd { return nil }

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.order)-1 {
				m.cursor++
			}
		case "enter":
			name := m.contextAt(m.cursor)
			return m, func() tea.Msg { return app.ContextChangedMsg{Name: name} }
		case "esc":
			return m, func() tea.Msg { return app.BackMsg{} }
		}
	}
	return m, nil
}

func (m *Model) contextAt(i int) string {
	if i < 0 || i >= len(m.order) {
		return ""
	}
	return m.order[i]
}

func (m *Model) View() string {
	var b strings.Builder
	b.WriteString("NAME              DESCRIPTION                       WAZUH SERVER\n")
	for i, name := range m.order {
		ctx := m.cfg.Contexts[name]
		server := ""
		if ctx.Wazuh != nil {
			server = fmt.Sprintf("%s:%d", ctx.Wazuh.Server, ctx.Wazuh.Port)
		}
		row := fmt.Sprintf("%-16s  %-32s  %s", name, ctx.Description, server)
		if i == m.cursor {
			row = styles.SelectedRow.Render(row)
		}
		fmt.Fprintln(&b, row)
	}
	return b.String()
}
