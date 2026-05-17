package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/statusbar"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/tabbar"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/views"
)

type Deps struct {
	Views       []views.View
	InitialView string

	// Status returns the current statusbar.State; injected so the root
	// model does not need to know about wazuh/config types.
	Status func() statusbar.State

	// PauseToggle is invoked on `p`; nil means no-op.
	PauseToggle func()
}

type Model struct {
	views       []views.View
	active      int
	status      func() statusbar.State
	pauseToggle func()
	// palette state will be added in Task 16.
}

func New(d Deps) *Model {
	m := &Model{
		views:       d.Views,
		status:      d.Status,
		pauseToggle: d.PauseToggle,
	}
	for i, v := range d.Views {
		if v.Name() == d.InitialView {
			m.active = i
		}
	}
	return m
}

func (m *Model) Active() views.View { return m.views[m.active] }

func (m *Model) Init() tea.Cmd {
	return m.Active().Init()
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.active = (m.active + 1) % len(m.views)
			return m, m.Active().Init()
		case "shift+tab":
			m.active = (m.active - 1 + len(m.views)) % len(m.views)
			return m, m.Active().Init()
		case "p":
			if m.pauseToggle != nil {
				m.pauseToggle()
			}
			return m, nil
		}
		if r := singleDigit(msg.String()); r > 0 && r <= len(m.views) {
			m.active = r - 1
			return m, m.Active().Init()
		}

	case SwitchViewMsg:
		for i, v := range m.views {
			if v.Name() == msg.Name {
				m.active = i
				return m, v.Init()
			}
		}
		return m, nil
	}

	// Delegate everything else to the active view.
	next, cmd := m.Active().Update(msg)
	if v, ok := next.(views.View); ok {
		m.views[m.active] = v
	}
	return m, cmd
}

func (m *Model) View() string {
	names := make([]string, len(m.views))
	for i, v := range m.views {
		names[i] = v.Name()
	}
	tab := tabbar.Render(names, m.active)

	var status string
	if m.status != nil {
		status = statusbar.Render(m.status())
	} else {
		status = statusbar.Render(statusbar.State{Refresh: time.Second})
	}
	body := m.Active().View()
	foot := footer.Render(m.Active().Hints())
	return strings.Join([]string{tab, status, body, foot}, "\n")
}

func singleDigit(s string) int {
	if len(s) != 1 {
		return 0
	}
	c := s[0]
	if c < '1' || c > '9' {
		return 0
	}
	return int(c - '0')
}
