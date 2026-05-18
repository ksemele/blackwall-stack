package agents

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

type ProcessesClient interface {
	ListProcesses(ctx context.Context, id string) ([]wazuh.Process, error)
}

type ProcessesLoadedMsg struct {
	Items []wazuh.Process
	Err   error
}

type ProcessesModel struct {
	client    ProcessesClient
	agentID   string
	items     []wazuh.Process
	loaded    bool
	err       string
	filter    string
	filtering bool
}

func NewProcesses(c ProcessesClient, agentID string) *ProcessesModel {
	return &ProcessesModel{client: c, agentID: agentID}
}

func (m *ProcessesModel) Init() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.ListProcesses(context.Background(), m.agentID)
		return ProcessesLoadedMsg{Items: items, Err: err}
	}
}

func (m *ProcessesModel) Update(msg tea.Msg) (subView, tea.Cmd) {
	switch msg := msg.(type) {
	case ProcessesLoadedMsg:
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		m.items = msg.Items
		m.loaded = true
		m.err = ""

	case tea.KeyMsg:
		if !m.filtering {
			if msg.String() == "/" {
				m.filtering = true
				m.filter = ""
			}
		} else {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filter = ""
			case "enter":
				m.filtering = false
			case "backspace":
				if len(m.filter) > 0 {
					_, size := utf8.DecodeLastRuneInString(m.filter)
					m.filter = m.filter[:len(m.filter)-size]
				}
			default:
				if msg.Type == tea.KeyRunes {
					m.filter += string(msg.Runes)
				}
			}
		}
	}
	return m, nil
}

func (m *ProcessesModel) CapturesKeys() bool { return m.filtering }

func (m *ProcessesModel) View() string {
	if m.err != "" {
		return styles.Error.Render("⚠ "+m.err) + "\n"
	}
	if !m.loaded {
		return styles.Dim.Render("loading…") + "\n"
	}

	var b strings.Builder

	fmt.Fprintf(&b, "%-7s  %-7s  %-10s  %-10s  %-20s  %s\n",
		"PID", "PPID", "USER", "RSS(KB)", "NAME", "COMMAND")
	b.WriteString(strings.Repeat("─", 80) + "\n")

	lower := strings.ToLower(m.filter)
	shown := 0
	for _, p := range m.items {
		if lower != "" {
			if !strings.Contains(strings.ToLower(p.Name), lower) &&
				!strings.Contains(strings.ToLower(p.Cmd), lower) &&
				!strings.Contains(strings.ToLower(p.Euser), lower) {
				continue
			}
		}
		fmt.Fprintf(&b, "%-7d  %-7d  %-10s  %-10d  %-20s  %s\n",
			p.PID, p.PPID, truncate(p.Euser, 10), p.Resident,
			truncate(p.Name, 20), truncate(p.Cmd, 50))
		shown++
	}

	if m.filtering {
		fmt.Fprintf(&b, "\nFilter: %s_\n", m.filter)
	} else if m.filter != "" {
		fmt.Fprintf(&b, "\nFilter: %s  (press / to edit, esc to clear)\n", m.filter)
	}

	fmt.Fprintf(&b, "%d/%d\n", shown, len(m.items))

	return b.String()
}
