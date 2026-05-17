package agents

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

type DetailModel struct {
	client  Client
	agentID string
	detail  wazuh.AgentDetail
	err     string
}

func NewDetail(c Client, id string) *DetailModel {
	return &DetailModel{client: c, agentID: id}
}

func (m *DetailModel) Name() string { return ":agent-detail" }

func (m *DetailModel) Hints() []footer.Hint {
	return []footer.Hint{
		{Key: "esc", Desc: "back"},
		{Key: "r", Desc: "refresh"},
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
	switch msg := msg.(type) {
	case DetailLoadedMsg:
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		m.err = ""
		m.detail = msg.Detail

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "left":
			return m, func() tea.Msg { return app.BackMsg{} }
		case "r":
			return m, m.Init()
		}
	}
	return m, nil
}

func (m *DetailModel) View() string {
	if m.err != "" {
		return fmt.Sprintf("⚠ %s\n", m.err)
	}
	a := m.detail.Agent
	lastSeen := wazuh.FormatTimestamp(a.LastKeepAlive, "2006-01-02 15:04:05")
	registered := wazuh.FormatTimestamp(a.DateAdd, "2006-01-02 15:04:05")
	osStr := strings.TrimSpace(a.OS.Name + " " + a.OS.Version)
	if osStr == "" {
		osStr = "—"
	}
	return fmt.Sprintf(
		"Agent %s — %s\n"+
			"──────────────────────────────\n"+
			"Status:     %s\n"+
			"IP:         %s\n"+
			"Version:    %s\n"+
			"OS:         %s\n"+
			"Last seen:  %s\n"+
			"Registered: %s\n"+
			"Manager:    %s\n",
		a.ID, a.Name, a.Status, a.IP, a.Version, osStr, lastSeen, registered, a.Manager,
	)
}
