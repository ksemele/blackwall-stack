// Package contexts implements the `:contexts` k9s-style picker.
package contexts

import (
	"context"
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/table"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

// reachability states for the circle indicator.
type reachState int

const (
	reachPending reachState = iota // probing in progress
	reachUp                        // responded
	reachDown                      // unreachable
	reachSSH                       // behind an ssh tunnel: not probed (would need ssh)
)

// ProbeResultMsg carries the result of a single context probe.
type ProbeResultMsg struct {
	Name  string
	Reach reachState
}

type Model struct {
	cfg    config.Config
	order  []string
	cursor int
	reach  map[string]reachState // per-context reachability cache
	width  int
}

func New(cfg config.Config) *Model {
	names := make([]string, 0, len(cfg.Contexts))
	for n := range cfg.Contexts {
		names = append(names, n)
	}
	sort.Strings(names)
	reach := make(map[string]reachState, len(names))
	cursor := 0
	for i, n := range names {
		reach[n] = reachPending
		if n == cfg.Current {
			cursor = i
		}
	}
	return &Model{cfg: cfg, order: names, reach: reach, cursor: cursor}
}

// UpdateCurrent refreshes the active-context pointer so the cursor
// tracks the newly selected context on next view open.
func (m *Model) UpdateCurrent(name string) {
	m.cfg.Current = name
	for i, n := range m.order {
		if n == name {
			m.cursor = i
			break
		}
	}
}

func (m *Model) Name() string { return ":contexts" }

func (m *Model) Hints() []footer.Hint {
	return []footer.Hint{
		{Key: "↑↓", Desc: "nav"},
		{Key: "↵", Desc: "switch"},
		{Key: "ctrl+r", Desc: "re-probe"},
		{Key: "?", Desc: "help"},
	}
}

// Init fires async probes for every context with a Wazuh subsystem.
func (m *Model) Init() tea.Cmd {
	// Reset all to pending on each open/re-init.
	for _, n := range m.order {
		m.reach[n] = reachPending
	}
	return m.probeAllCmd()
}

func (m *Model) probeAllCmd() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.order))
	for _, name := range m.order {
		ctx := m.cfg.Contexts[name]
		if ctx.Wazuh == nil {
			cmds = append(cmds, func() tea.Msg {
				return ProbeResultMsg{Name: name, Reach: reachDown}
			})
			continue
		}
		if ctx.SSH != nil {
			// server/port are as seen from the ssh host; probing them
			// locally would hit the wrong endpoint.
			cmds = append(cmds, func() tea.Msg {
				return ProbeResultMsg{Name: name, Reach: reachSSH}
			})
			continue
		}
		sub := *ctx.Wazuh
		n := name // capture
		cmds = append(cmds, func() tea.Msg {
			err := wazuh.Probe(context.Background(), sub)
			r := reachUp
			if err != nil {
				r = reachDown
			}
			return ProbeResultMsg{Name: n, Reach: r}
		})
	}
	return tea.Batch(cmds...)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

	case ProbeResultMsg:
		m.reach[msg.Name] = msg.Reach
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
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
		case "ctrl+r":
			for _, n := range m.order {
				m.reach[n] = reachPending
			}
			return m, m.probeAllCmd()
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

func (m *Model) circle(name string) string {
	switch m.reach[name] {
	case reachUp:
		return styles.Active.Render("●")
	case reachDown:
		return styles.Disconnected.Render("●")
	case reachSSH:
		return styles.Pending.Render("○")
	default: // reachPending
		return styles.Pending.Render("●")
	}
}

// gutter is the visible width of the leading circle + space prefix that sits
// outside the table grid (kept ungrayed so reachability colour stays visible
// when the row is highlighted).
const gutter = 2

var ctxCols = []table.Col{
	{Title: "NAME", Min: 16, Flex: true},
	{Title: "DESCRIPTION", Min: 32, Flex: true},
	{Title: "WAZUH SERVER", Min: 20, Flex: true},
}

func (m *Model) View() string {
	var b strings.Builder

	widths := table.Widths(ctxCols, max(m.width-gutter, 0))

	b.WriteString(strings.Repeat(" ", gutter))
	b.WriteString(table.Header(ctxCols, widths))
	b.WriteByte('\n')

	for i, name := range m.order {
		ctx := m.cfg.Contexts[name]
		server := ""
		if ctx.Wazuh != nil {
			server = fmt.Sprintf("%s:%d", ctx.Wazuh.Server, ctx.Wazuh.Port)
			if ctx.SSH != nil {
				server += " via ssh " + ctx.SSH.Host
			}
		}
		circle := m.circle(name)
		textPart := table.Row(widths, []string{name, ctx.Description, server})
		var row string
		if i == m.cursor {
			row = circle + " " + styles.SelectedRow.Render(textPart)
		} else if m.reach[name] == reachDown {
			row = circle + " " + styles.RowDown.Render(textPart)
		} else {
			row = circle + " " + textPart
		}
		fmt.Fprintln(&b, row)
	}
	return b.String()
}
