package agents

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/scroll"
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
	sl        scroll.List
}

func NewProcesses(c ProcessesClient, agentID string) *ProcessesModel {
	return &ProcessesModel{
		client:  c,
		agentID: agentID,
		sl:      scroll.List{HeaderRows: 2, FooterRows: 2},
	}
}

// SetSize implements sizedView so DetailModel can propagate window resize.
func (m *ProcessesModel) SetSize(width, height int) {
	m.sl.SetSize(height)
}

// Hints implements hintProvider.
func (m *ProcessesModel) Hints() []footer.Hint {
	return []footer.Hint{
		{Key: "j/k", Desc: "scroll"},
		{Key: "ctrl+d/u", Desc: "half page"},
		{Key: "g/G", Desc: "top/bottom"},
		{Key: "/", Desc: "filter"},
	}
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
		m.sl.Reset()

	case tea.KeyMsg:
		if !m.filtering {
			if msg.String() == "/" {
				m.filtering = true
				m.filter = ""
				m.sl.Reset()
				return m, nil
			}
			// Delegate scroll keys to scroll.List.
			if m.sl.HandleKey(msg, len(m.filtered())) {
				return m, nil
			}
		} else {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filter = ""
				m.sl.Reset()
			case "enter":
				m.filtering = false
				m.sl.Reset()
			case "backspace":
				if len(m.filter) > 0 {
					_, size := utf8.DecodeLastRuneInString(m.filter)
					m.filter = m.filter[:len(m.filter)-size]
				}
			default:
				if msg.Type == tea.KeyRunes {
					m.filter += string(msg.Runes)
					m.sl.Reset()
				}
			}
		}
	}
	return m, nil
}

func (m *ProcessesModel) CapturesKeys() bool { return m.filtering }

// filtered returns the subset of items matching the current filter.
func (m *ProcessesModel) filtered() []wazuh.Process {
	lower := strings.ToLower(m.filter)
	if lower == "" {
		return m.items
	}
	out := make([]wazuh.Process, 0, len(m.items))
	for _, p := range m.items {
		if strings.Contains(strings.ToLower(p.Name), lower) ||
			strings.Contains(strings.ToLower(p.Cmd), lower) ||
			strings.Contains(strings.ToLower(p.Euser), lower) {
			out = append(out, p)
		}
	}
	return out
}

func (m *ProcessesModel) View() string {
	if m.err != "" {
		return styles.Error.Render("⚠ "+m.err) + "\n"
	}
	if !m.loaded {
		return styles.Dim.Render("loading…") + "\n"
	}

	headerBlock := fmt.Sprintf("%-7s  %-7s  %-10s  %-10s  %-20s  %s\n",
		"PID", "PPID", "USER", "RSS(KB)", "NAME", "COMMAND") +
		strings.Repeat("─", 80) + "\n"

	filtered := m.filtered()

	rows := make([]string, 0, len(filtered))
	for _, p := range filtered {
		rows = append(rows, fmt.Sprintf("%-7d  %-7d  %-10s  %-10d  %-20s  %s",
			p.PID, p.PPID, truncate(p.Euser, 10), p.Resident,
			truncate(p.Name, 20), truncate(p.Cmd, 50)))
	}

	// footerRows=2: one filter/blank line + one count+indicator line.
	var footerParts strings.Builder
	if m.filtering {
		fmt.Fprintf(&footerParts, "Filter: %s_\n", m.filter)
	} else if m.filter != "" {
		fmt.Fprintf(&footerParts, "Filter: %s  (/ edit, esc clear)\n", m.filter)
	} else {
		footerParts.WriteString("\n")
	}
	fmt.Fprintf(&footerParts, "%d/%d  %s\n", len(filtered), len(m.items), m.sl.Indicator(len(rows)))

	return m.sl.Render(headerBlock, rows, footerParts.String())
}
