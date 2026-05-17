// Package agents implements the `:agents` list view and the detail drill-down.
package agents

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/tickcmd"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

// Client is the subset of *wazuh.Client this view uses.
type Client interface {
	ListAgents(ctx context.Context) ([]wazuh.Agent, error)
	GetAgent(ctx context.Context, id string) (wazuh.AgentDetail, error)
}

type Model struct {
	client          Client
	agents          []wazuh.Agent
	cursor          int
	paused          bool
	refreshInterval time.Duration
	lastErr         string
}

func New(c Client, refresh time.Duration) *Model {
	if refresh <= 0 {
		refresh = 3 * time.Second
	}
	return &Model{client: c, refreshInterval: refresh}
}

func (m *Model) Name() string { return ":agents" }

func (m *Model) Hints() []footer.Hint {
	return []footer.Hint{
		{Key: "↑↓", Desc: "nav"},
		{Key: "↵", Desc: "details"},
		{Key: "p", Desc: "pause"},
		{Key: "/", Desc: "filter"},
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(m.fetchCmd(), tickcmd.Every(m.refreshInterval))
}

func (m *Model) SetAgents(a []wazuh.Agent) { m.agents = a }
func (m *Model) SetPaused(p bool)          { m.paused = p }

type AgentsLoadedMsg struct {
	Agents []wazuh.Agent
	Err    error
}

func (m *Model) fetchCmd() tea.Cmd {
	if m.paused {
		return nil
	}
	return func() tea.Msg {
		agents, err := m.client.ListAgents(context.Background())
		return AgentsLoadedMsg{Agents: agents, Err: err}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickcmd.TickMsg:
		return m, tea.Batch(m.fetchCmd(), tickcmd.Every(m.refreshInterval))

	case AgentsLoadedMsg:
		if msg.Err != nil {
			m.lastErr = msg.Err.Error()
			return m, nil
		}
		m.lastErr = ""
		m.agents = msg.Agents
		if m.cursor >= len(m.agents) {
			m.cursor = 0
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.agents)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.agents) == 0 {
				return m, nil
			}
			id := m.agents[m.cursor].ID
			return m, func() tea.Msg { return app.SwitchToDetailMsg{AgentID: id} }
		case "p":
			m.paused = !m.paused
		case "ctrl+r":
			return m, m.fetchCmd()
		}
	}
	return m, nil
}

func (m *Model) View() string {
	var b strings.Builder
	if m.lastErr != "" {
		fmt.Fprintf(&b, "⚠ %s\n", m.lastErr)
	}
	fmt.Fprintf(&b, "%-4s  %-20s  %-16s  %-12s  %s\n", "ID", "NAME", "IP", "STATUS", "LAST SEEN")
	for i, a := range m.agents {
		lastSeen := wazuh.FormatTimestamp(a.LastKeepAlive, "2006-01-02 15:04")
		row := fmt.Sprintf("%-4s  %-20s  %-16s  %-12s  %s",
			a.ID, a.Name, a.IP, a.Status, lastSeen)
		if i == m.cursor {
			row = styles.SelectedRow.Render(row)
		} else if a.Status != "active" {
			row = styles.RowDown.Render(row)
		}
		fmt.Fprintln(&b, row)
	}
	if len(m.agents) == 0 && m.lastErr == "" {
		b.WriteString("\nNo agents.\n")
	}
	return b.String()
}
