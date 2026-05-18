package agents

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/tabbar"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

// statusStyled returns the agent status colored to match the agents-list view:
// green=active, yellow=pending, grey=never_connected, red=anything else.
func statusStyled(status string) string {
	switch status {
	case "active":
		return styles.Active.Render(status)
	case "pending":
		return styles.Pending.Render(status)
	case "never_connected":
		return styles.RowGhost.Render(status)
	default:
		return styles.Disconnected.Render(status)
	}
}

var detailTabs = []string{"Overview", "Hardware", "Top", "Packages", "Network", "Security"}

// tabIndex maps single-letter hotkeys to tab indices.
var tabIndex = map[string]int{
	"o": 0,
	"h": 1,
	"t": 2,
	"k": 3,
	"n": 4,
	"s": 5,
}

type DetailModel struct {
	client    Client
	agentID   string
	detail    wazuh.AgentDetail
	err       string
	activeTab int
	width     int
	subs      map[int]subView
}

func NewDetail(c Client, id string) *DetailModel {
	return &DetailModel{
		client:  c,
		agentID: id,
		subs: map[int]subView{
			1: NewHardware(c, id),
			2: NewProcesses(c, id),
			3: NewPackages(c, id),
			4: NewNetwork(c, id),
			5: NewSecurity(c, id),
		},
	}
}

func (m *DetailModel) Name() string { return ":agent-detail" }

func (m *DetailModel) Hints() []footer.Hint {
	return []footer.Hint{
		{Key: "o/h/t/k/n/s", Desc: "tabs"},
		{Key: "esc", Desc: "back"},
		{Key: "r", Desc: "refresh"},
		{Key: "?", Desc: "help"},
	}
}

func (m *DetailModel) SetDetail(d wazuh.AgentDetail) { m.detail = d }

type DetailLoadedMsg struct {
	Detail wazuh.AgentDetail
	Err    error
}

func (m *DetailModel) Init() tea.Cmd {
	return func() tea.Msg {
		d, err := m.client.GetAgent(context.Background(), m.agentID)
		return DetailLoadedMsg{Detail: d, Err: err}
	}
}

func (m *DetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		return m, nil
	}

	switch msg := msg.(type) {
	case DetailLoadedMsg:
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		m.err = ""
		m.detail = msg.Detail

	case tea.KeyMsg:
		if sub, ok := m.subs[m.activeTab]; ok {
			if kc, ok := sub.(keyCapturer); ok && kc.CapturesKeys() {
				next, cmd := sub.Update(msg)
				m.subs[m.activeTab] = next
				return m, cmd
			}
		}
		if idx, ok := tabIndex[msg.String()]; ok {
			m.activeTab = idx
			if sub, hasSub := m.subs[idx]; hasSub {
				return m, sub.Init()
			}
			return m, nil
		}
		switch msg.String() {
		case "esc", "left":
			return m, func() tea.Msg { return app.BackMsg{} }
		case "r":
			return m, m.Init()
		}
	}

	if sub, ok := m.subs[m.activeTab]; ok {
		next, cmd := sub.Update(msg)
		m.subs[m.activeTab] = next
		return m, cmd
	}

	return m, nil
}

// renderOverview renders agent metadata (was the full View content before tabbar).
func (m *DetailModel) renderOverview() string {
	a := m.detail.Agent
	lastSeen := wazuh.FormatTimestamp(a.LastKeepAlive, "2006-01-02 15:04:05")
	registered := wazuh.FormatTimestamp(a.DateAdd, "2006-01-02 15:04:05")
	osStr := strings.TrimSpace(a.OS.Name + " " + a.OS.Version)
	if osStr == "" {
		osStr = "—"
	}
	return fmt.Sprintf(
		"──────────────────────────────\n"+
			"Status:     %s\n"+
			"IP:         %s\n"+
			"Version:    %s\n"+
			"OS:         %s\n"+
			"Last seen:  %s\n"+
			"Registered: %s\n"+
			"Manager:    %s\n",
		statusStyled(a.Status), a.IP, a.Version, osStr, lastSeen, registered, a.Manager,
	)
}

func (m *DetailModel) renderActiveTab() string {
	if sub, ok := m.subs[m.activeTab]; ok {
		return sub.View()
	}
	switch m.activeTab {
	case 0:
		return m.renderOverview()
	case 2:
		return "Top — coming soon\n"
	case 3:
		return "Packages — coming soon\n"
	default:
		return ""
	}
}

func (m *DetailModel) View() string {
	a := m.detail.Agent
	header := fmt.Sprintf("Agent %s — %s  %s\n", a.ID, a.Name, statusStyled(a.Status))
	tabs := tabbar.Render(detailTabs, m.activeTab, m.width, "")

	if m.err != "" {
		return header + tabs + "\n" + fmt.Sprintf("⚠ %s\n", m.err)
	}

	return header + tabs + "\n" + m.renderActiveTab()
}
