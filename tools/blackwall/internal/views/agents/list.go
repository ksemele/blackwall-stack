// Package agents implements the `:agents` list view and the detail drill-down.
package agents

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/tickcmd"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/table"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

// Client is the subset of *wazuh.Client this view uses.
type Client interface {
	ListAgents(ctx context.Context) ([]wazuh.Agent, error)
	GetAgent(ctx context.Context, id string) (wazuh.AgentDetail, error)
	GetHardware(ctx context.Context, id string) (wazuh.Hardware, error)
	GetOSInfo(ctx context.Context, id string) (wazuh.OSInfo, error)
	ListProcesses(ctx context.Context, id string) ([]wazuh.Process, error)
	ListPackages(ctx context.Context, id string) ([]wazuh.Package, error)
	ListNetiface(ctx context.Context, id string) ([]wazuh.Netiface, error)
	ListNetaddr(ctx context.Context, id string) ([]wazuh.Netaddr, error)
	ListPorts(ctx context.Context, id string) ([]wazuh.Port, error)
	GetSyscheckLastScan(ctx context.Context, id string) (wazuh.SyscheckScan, error)
	ListSyscheckEntries(ctx context.Context, id string) ([]wazuh.SyscheckEntry, error)
	GetRootcheckLastScan(ctx context.Context, id string) (wazuh.RootcheckScan, error)
	ListRootcheckFindings(ctx context.Context, id string) ([]wazuh.RootcheckFinding, error)
	ListSCAPolicies(ctx context.Context, id string) ([]wazuh.SCAPolicy, error)
}

type Model struct {
	client          Client
	agents          []wazuh.Agent
	cursor          int
	paused          bool
	refreshInterval time.Duration
	lastErr         string
	width           int
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
		{Key: "?", Desc: "help"},
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
	case tea.WindowSizeMsg:
		m.width = msg.Width
		return m, nil

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

// Last-seen layouts for table rows: local date and time, UTC time only.
const (
	shortLocal = "2006-01-02 15:04"
	shortUTC   = "15:04Z"
)

// dockerTag follows the name of agents in the wazuh.ContainerGroup group.
const dockerTag = "[docker]"

// rowStyle colours a whole agents row by status; active rows stay default.
func rowStyle(status string) lipgloss.Style {
	switch status {
	case "active":
		return lipgloss.NewStyle()
	case "pending":
		return styles.RowPending
	case "never_connected":
		return styles.RowGhost
	default:
		return styles.RowDown
	}
}

var agentCols = []table.Col{
	{Title: "ID", Min: 4},
	{Title: "NAME", Min: 20, Flex: true},
	{Title: "IP", Min: 16},
	{Title: "STATUS", Min: 12},
	{Title: "WAZUH", Min: 14},
	{Title: "OS", Min: 16, Flex: true},
	{Title: "LAST SEEN", Min: 24, Flex: true},
}

func (m *Model) View() string {
	var b strings.Builder
	if m.lastErr != "" {
		fmt.Fprintf(&b, "⚠ %s\n", m.lastErr)
	}

	widths := table.Widths(agentCols, m.width)
	b.WriteString(table.Header(agentCols, widths))
	b.WriteByte('\n')

	for i, a := range m.agents {
		lastSeen := wazuh.FormatLocalUTC(a.LastKeepAlive, shortLocal, shortUTC)
		osStr := strings.TrimSpace(a.OS.Name + " " + a.OS.Version)
		if osStr == "" {
			osStr = "—"
		}
		version := a.Version
		if version == "" {
			version = "—"
		}
		values := []string{a.ID, a.Name, a.IP, a.Status, version, osStr, lastSeen}
		var row string
		if i == m.cursor {
			// The full-row highlight wins; the badge stays as plain text so its
			// own colours do not cut the highlight in half.
			if a.InContainer() {
				values[1] += " " + dockerTag
			}
			row = styles.SelectedRow.Render(table.Row(widths, values))
		} else {
			// Style cell by cell: a styled badge inside a row-wide style would
			// reset the colour of every column after it.
			st := rowStyle(a.Status)
			for j := range values {
				values[j] = st.Render(values[j])
			}
			if a.InContainer() {
				values[1] += " " + styles.DockerBadge.Render(dockerTag)
			}
			row = table.Row(widths, values)
		}
		fmt.Fprintln(&b, row)
	}
	if len(m.agents) == 0 && m.lastErr == "" {
		b.WriteString("\nNo agents.\n")
	}
	return b.String()
}
