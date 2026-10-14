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

// containerWarning explains what the DOCKER badge costs: container runtimes
// share the host kernel, and kernels without audit support (Docker Desktop,
// OrbStack) leave auditd unable to start.
const containerWarning = "⚠ Container host: no kernel audit — auditd rules do not fire. " +
	"FIM, rootcheck, SCA and inventory work."

var detailTabs = []string{"Overview", "Hardware", "Top", "Packages", "Network", "Security", "Sessions"}

// detailTabKeys are the upper-case hotkey labels shown in the tabbar prefix.
// IMPORTANT: keep in sync with tabIndex below — same order, same length.
var detailTabKeys = []string{"O", "H", "T", "K", "N", "S", "E"}

// tabIndex maps single-letter hotkeys to tab indices.
var tabIndex = map[string]int{
	"o": 0,
	"h": 1,
	"t": 2,
	"k": 3,
	"n": 4,
	"s": 5,
	"e": 6,
}

type DetailModel struct {
	client    Client
	agentID   string
	detail    wazuh.AgentDetail
	err       string
	activeTab int
	width     int
	height    int
	subs      map[int]subView
}

func NewDetail(c Client, idx AlertSearcher, id string) *DetailModel {
	return &DetailModel{
		client:  c,
		agentID: id,
		subs: map[int]subView{
			1: NewHardware(c, id),
			2: NewProcesses(c, id),
			3: NewPackages(c, id),
			4: NewNetwork(c, id),
			5: NewSecurity(c, id),
			6: NewSessions(idx, id),
		},
	}
}

func (m *DetailModel) Name() string { return ":agent-detail" }

func (m *DetailModel) Hints() []footer.Hint {
	base := []footer.Hint{
		{Key: "o/h/t/k/n/s/e", Desc: "tabs"},
		{Key: "l", Desc: "alerts"},
		{Key: "esc", Desc: "back"},
		{Key: "r", Desc: "refresh"},
		{Key: "?", Desc: "help"},
	}
	// Let active sub-view inject extra hints (e.g. scroll keys).
	if sub, ok := m.subs[m.activeTab]; ok {
		if kc, ok2 := sub.(keyCapturer); ok2 && kc.CapturesKeys() {
			// In filter mode sub-view owns the keyboard; don't add scroll hints.
			return base
		}
		if hp, ok2 := sub.(hintProvider); ok2 {
			return append(base, hp.Hints()...)
		}
	}
	return base
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
		m.height = ws.Height
		// ws is the body area (the app already took its chrome); the detail
		// header takes two more rows: agent name and the tab bar.
		subHeight := max(ws.Height-2, 1)
		for i, sub := range m.subs {
			if sv, ok := sub.(sizedView); ok {
				sv.SetSize(ws.Width, subHeight)
				m.subs[i] = sub
			}
		}
		// Don't return early — fall through so any other handler gets the message.
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
		case "l":
			id, name := m.agentID, m.detail.Name
			return m, func() tea.Msg { return app.ShowAgentAlertsMsg{AgentID: id, AgentName: name} }
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
	const local, utc = "2006-01-02 15:04:05 MST", "2006-01-02 15:04:05 UTC"
	lastSeen := wazuh.FormatLocalUTC(a.LastKeepAlive, local, utc)
	registered := wazuh.FormatLocalUTC(a.DateAdd, local, utc)
	osStr := strings.TrimSpace(a.OS.Name + " " + a.OS.Version)
	if osStr == "" {
		osStr = "—"
	}
	warn := ""
	if a.InContainer() {
		warn = styles.Error.Render(containerWarning) + "\n"
	}
	return warn + fmt.Sprintf(
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
	header := fmt.Sprintf("Agent %s — %s  %s", a.ID, a.Name, statusStyled(a.Status))
	if a.InContainer() {
		header += "  " + styles.DockerBadge.Render(" DOCKER ")
	}
	header += "\n"
	tabs := tabbar.RenderWithKeys(detailTabs, detailTabKeys, m.activeTab, m.width, "")

	if m.err != "" {
		return header + tabs + "\n" + fmt.Sprintf("⚠ %s\n", m.err)
	}

	return header + tabs + "\n" + m.renderActiveTab()
}
