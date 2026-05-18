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

type PackagesClient interface {
	ListPackages(ctx context.Context, id string) ([]wazuh.Package, error)
}

type PackagesLoadedMsg struct {
	Items []wazuh.Package
	Err   error
}

type PackagesModel struct {
	client    PackagesClient
	agentID   string
	items     []wazuh.Package
	loaded    bool
	err       string
	filter    string
	filtering bool
}

func NewPackages(c PackagesClient, agentID string) *PackagesModel {
	return &PackagesModel{client: c, agentID: agentID}
}

func (m *PackagesModel) Init() tea.Cmd {
	return func() tea.Msg {
		items, err := m.client.ListPackages(context.Background(), m.agentID)
		return PackagesLoadedMsg{Items: items, Err: err}
	}
}

func (m *PackagesModel) Update(msg tea.Msg) (subView, tea.Cmd) {
	switch msg := msg.(type) {
	case PackagesLoadedMsg:
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

func (m *PackagesModel) CapturesKeys() bool { return m.filtering }

func (m *PackagesModel) View() string {
	if m.err != "" {
		return styles.Error.Render("⚠ "+m.err) + "\n"
	}
	if !m.loaded {
		return styles.Dim.Render("loading…") + "\n"
	}

	var b strings.Builder

	fmt.Fprintf(&b, "%-36s  %-20s  %-16s  %s\n",
		"NAME", "VERSION", "VENDOR", "INSTALL TIME")
	b.WriteString(strings.Repeat("─", 90) + "\n")

	lower := strings.ToLower(m.filter)
	shown := 0
	for _, p := range m.items {
		if lower != "" {
			if !strings.Contains(strings.ToLower(p.Name), lower) &&
				!strings.Contains(strings.ToLower(p.Vendor), lower) &&
				!strings.Contains(strings.ToLower(p.Description), lower) {
				continue
			}
		}
		fmt.Fprintf(&b, "%-36s  %-20s  %-16s  %s\n",
			truncate(p.Name, 36), truncate(p.Version, 20),
			truncate(p.Vendor, 16), p.InstallTime)
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
