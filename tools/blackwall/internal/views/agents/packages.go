package agents

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
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
	sl        scrollist
}

func NewPackages(c PackagesClient, agentID string) *PackagesModel {
	return &PackagesModel{
		client: c,
		agentID: agentID,
		sl: scrollist{headerRows: 2, footerRows: 2},
	}
}

// SetSize implements sizedView so DetailModel can propagate window resize.
func (m *PackagesModel) SetSize(width, height int) {
	m.sl.SetSize(height)
}

// Hints implements hintProvider.
func (m *PackagesModel) Hints() []footer.Hint {
	return []footer.Hint{
		{Key: "j/k", Desc: "scroll"},
		{Key: "ctrl+d/u", Desc: "half page"},
		{Key: "g/G", Desc: "top/bottom"},
		{Key: "/", Desc: "filter"},
	}
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
		m.sl.scroll = 0

	case tea.KeyMsg:
		if !m.filtering {
			if msg.String() == "/" {
				m.filtering = true
				m.filter = ""
				m.sl.scroll = 0
				return m, nil
			}
			// Delegate scroll keys to scrollist.
			if m.sl.HandleKey(msg, len(m.filtered())) {
				return m, nil
			}
		} else {
			switch msg.String() {
			case "esc":
				m.filtering = false
				m.filter = ""
				m.sl.scroll = 0
			case "enter":
				m.filtering = false
				m.sl.scroll = 0
			case "backspace":
				if len(m.filter) > 0 {
					_, size := utf8.DecodeLastRuneInString(m.filter)
					m.filter = m.filter[:len(m.filter)-size]
				}
			default:
				if msg.Type == tea.KeyRunes {
					m.filter += string(msg.Runes)
					m.sl.scroll = 0
				}
			}
		}
	}
	return m, nil
}

func (m *PackagesModel) CapturesKeys() bool { return m.filtering }

// filtered returns the subset of items matching the current filter.
func (m *PackagesModel) filtered() []wazuh.Package {
	lower := strings.ToLower(m.filter)
	if lower == "" {
		return m.items
	}
	out := make([]wazuh.Package, 0, len(m.items))
	for _, p := range m.items {
		if strings.Contains(strings.ToLower(p.Name), lower) ||
			strings.Contains(strings.ToLower(p.Vendor), lower) ||
			strings.Contains(strings.ToLower(p.Description), lower) {
			out = append(out, p)
		}
	}
	return out
}

func (m *PackagesModel) View() string {
	if m.err != "" {
		return styles.Error.Render("⚠ "+m.err) + "\n"
	}
	if !m.loaded {
		return styles.Dim.Render("loading…") + "\n"
	}

	headerBlock := fmt.Sprintf("%-36s  %-20s  %-16s  %s\n",
		"NAME", "VERSION", "VENDOR", "INSTALL TIME") +
		strings.Repeat("─", 90) + "\n"

	filtered := m.filtered()

	rows := make([]string, 0, len(filtered))
	for _, p := range filtered {
		rows = append(rows, fmt.Sprintf("%-36s  %-20s  %-16s  %s",
			truncate(p.Name, 36), truncate(p.Version, 20),
			truncate(p.Vendor, 16), p.InstallTime))
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
