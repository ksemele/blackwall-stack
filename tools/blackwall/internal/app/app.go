package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/statusbar"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
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

	// SwitchContext is called when the user selects a context.
	// It must persist the change and return a tea.Cmd that fires
	// ContextSwitchedMsg when done.
	SwitchContext func(name string) tea.Cmd
}

type Model struct {
	views       []views.View
	active      int
	status      func() statusbar.State
	pauseToggle func()
	switchCtx   func(name string) tea.Cmd

	footerMsg string // transient confirmation shown in statusbar area

	paletteOpen  bool
	paletteInput string
	paletteErr   string
}

func New(d Deps) *Model {
	m := &Model{
		views:       d.Views,
		status:      d.Status,
		pauseToggle: d.PauseToggle,
		switchCtx:   d.SwitchContext,
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
		if m.paletteOpen {
			switch msg.String() {
			case "esc":
				m.paletteOpen = false
				m.paletteInput = ""
				m.paletteErr = ""
				return m, nil
			case "enter":
				names := viewNames(m.views)
				out, err := parsePalette(m.paletteInput, names)
				if err != nil {
					m.paletteErr = err.Error()
					return m, nil
				}
				m.paletteOpen = false
				m.paletteInput = ""
				m.paletteErr = ""
				if _, ok := out.(QuitMsg); ok {
					return m, tea.Quit
				}
				return m.Update(out)
			case "backspace":
				if n := len(m.paletteInput); n > 0 {
					m.paletteInput = m.paletteInput[:n-1]
				}
				return m, nil
			default:
				if msg.Type == tea.KeyRunes {
					m.paletteInput += string(msg.Runes)
				}
				return m, nil
			}
		}
		if msg.String() == ":" {
			m.paletteOpen = true
			m.paletteInput = ":"
			m.paletteErr = ""
			return m, nil
		}
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

	case ContextChangedMsg:
		if m.switchCtx != nil {
			return m, m.switchCtx(msg.Name)
		}
		return m, nil

	case ContextSwitchedMsg:
		if msg.Err != nil {
			m.footerMsg = "⚠ context switch failed: " + msg.Err.Error()
			return m, nil
		}
		m.footerMsg = ""
		// Navigate to agents view.
		for i, v := range m.views {
			if v.Name() == ":agents" {
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
	if m.paletteOpen {
		body += "\n\n> " + m.paletteInput
		if m.paletteErr != "" {
			body += "\n" + styles.Error.Render(m.paletteErr)
		}
	}
	foot := footer.Render(m.Active().Hints())
	parts := []string{tab, status}
	if m.footerMsg != "" {
		parts = append(parts, styles.Active.Render(m.footerMsg))
	}
	parts = append(parts, body, foot)
	return strings.Join(parts, "\n")
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

func viewNames(vs []views.View) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Name()
	}
	return out
}
