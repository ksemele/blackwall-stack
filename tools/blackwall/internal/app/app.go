package app

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/statusbar"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/tabbar"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/views"
)

type Deps struct {
	Views       []views.View
	InitialView string

	// Version is displayed right-aligned in the tab bar (e.g. "v0.0.1").
	Version string

	// Status returns the current statusbar.State; injected so the root
	// model does not need to know about wazuh/config types.
	Status func() statusbar.State

	// PauseToggle is invoked on `p`; nil means no-op.
	PauseToggle func()

	// SwitchContext is called when the user selects a context.
	// It must persist the change and return a tea.Cmd that fires
	// ContextSwitchedMsg when done.
	SwitchContext func(name string) tea.Cmd

	// OnRefresh is called (without consuming the key) on `r` / ctrl+r, before
	// the active view handles it; main uses it to retry a down ssh tunnel.
	OnRefresh func()

	// HelpView is the view pushed when the user presses `?`.
	// If nil, the `?` key is a no-op.
	HelpView views.View
}

type Model struct {
	views       []views.View
	active      int
	version     string
	width       int
	height      int
	status      func() statusbar.State
	pauseToggle func()
	switchCtx   func(name string) tea.Cmd
	onRefresh   func()
	helpView    views.View

	// helpOpen is true while the help overlay is shown.
	helpOpen bool

	footerMsg string // transient confirmation shown in statusbar area

	paletteOpen  bool
	paletteInput string
	paletteErr   string
}

func New(d Deps) *Model {
	m := &Model{
		views:       d.Views,
		version:     d.Version,
		status:      d.Status,
		pauseToggle: d.PauseToggle,
		switchCtx:   d.SwitchContext,
		onRefresh:   d.OnRefresh,
		helpView:    d.HelpView,
	}
	for i, v := range d.Views {
		if v.Name() == d.InitialView {
			m.active = i
		}
	}
	return m
}

func (m *Model) Active() views.View {
	if m.helpOpen && m.helpView != nil {
		return m.helpView
	}
	return m.views[m.active]
}

func (m *Model) Init() tea.Cmd {
	return m.Active().Init()
}

// chromeRows is what View() draws around the active view: tab bar, status
// line and rule on top, rule and key hints at the bottom.
const chromeRows = 5

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Track terminal size for right-aligned layout and footer pinning.
	// Broadcast the size to every view (including the help overlay) so an
	// inactive view has the correct width on its first render after
	// activation.
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = ws.Width
		m.height = ws.Height
		// Views get the body area, not the whole terminal: the chrome rows
		// rendered in View() are not theirs to fill.
		body := tea.WindowSizeMsg{Width: ws.Width, Height: max(ws.Height-chromeRows, 1)}
		for i, v := range m.views {
			next, _ := v.Update(body)
			if vv, ok := next.(views.View); ok {
				m.views[i] = vv
			}
		}
		if m.helpView != nil {
			next, _ := m.helpView.Update(body)
			if vv, ok := next.(views.View); ok {
				m.helpView = vv
			}
		}
		return m, nil
	}

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

		// While help is open, delegate all keys to the help view so it
		// can handle its own `q` / `ESC` → BackMsg.
		if m.helpOpen && m.helpView != nil {
			next, cmd := m.helpView.Update(msg)
			if v, ok := next.(views.View); ok {
				m.helpView = v
			}
			return m, cmd
		}

		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if kc, ok := m.Active().(views.KeyCapturer); ok && kc.CapturesKeys() {
			next, cmd := m.Active().Update(msg)
			if v, ok := next.(views.View); ok {
				m.views[m.active] = v
			}
			return m, cmd
		}

		if s := msg.String(); (s == "r" || s == "ctrl+r") && m.onRefresh != nil {
			m.onRefresh() // the view still gets the key below
		}
		if msg.String() == ":" {
			m.paletteOpen = true
			m.paletteInput = ":"
			m.paletteErr = ""
			return m, nil
		}
		switch msg.String() {
		case "?":
			if m.helpView != nil {
				m.helpOpen = true
				return m, m.helpView.Init()
			}
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			m.active = (m.active + 1) % len(m.views)
			return m, m.Active().Init()
		case "shift+tab":
			m.active = (m.active - 1 + len(m.views)) % len(m.views)
			return m, m.Active().Init()
		case "p":
			// Only consume `p` when a pause hook is wired; otherwise the
			// active view owns pause (the agents list and :alerts do).
			if m.pauseToggle != nil {
				m.pauseToggle()
				return m, nil
			}
		}
		if r := singleDigit(msg.String()); r > 0 && r <= len(m.views) {
			m.active = r - 1
			return m, m.Active().Init()
		}

	case BackMsg:
		if m.helpOpen {
			m.helpOpen = false
			return m, nil
		}

	case SwitchViewMsg:
		for i, v := range m.views {
			if v.Name() == msg.Name {
				m.active = i
				return m, v.Init()
			}
		}
		return m, nil

	case ShowAgentAlertsMsg:
		for i, v := range m.views {
			if v.Name() == ":alerts" {
				m.active = i
				next, cmd := v.Update(msg)
				if vv, ok := next.(views.View); ok {
					m.views[i] = vv
				}
				return m, cmd
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
		if m.helpOpen && m.helpView != nil {
			m.helpView = v
		} else {
			m.views[m.active] = v
		}
	}
	return m, cmd
}

func (m *Model) View() string {
	names := make([]string, len(m.views))
	for i, v := range m.views {
		names[i] = v.Name()
	}
	tab := tabbar.Render(names, m.active, m.width, m.version)

	var status string
	if m.status != nil {
		status = statusbar.Render(m.status())
	} else {
		status = statusbar.Render(statusbar.State{Refresh: time.Second})
	}

	// Horizontal rule used to bracket the body — one above (separating from
	// the header chrome) and one below (separating from the footer).
	ruleWidth := m.width
	if ruleWidth <= 0 {
		ruleWidth = 80 // sane fallback before first WindowSizeMsg
	}
	rule := styles.HRule.Render(strings.Repeat("─", ruleWidth))

	body := m.Active().View()
	if m.paletteOpen {
		body += "\n\n> " + m.paletteInput
		if m.paletteErr != "" {
			body += "\n" + styles.Error.Render(m.paletteErr)
		}
	}
	body = strings.TrimRight(body, "\n")

	foot := footer.Render(m.Active().Hints())

	headerParts := []string{tab, status, rule}
	if m.footerMsg != "" {
		headerParts = append(headerParts, styles.Active.Render(m.footerMsg))
	}
	header := strings.Join(headerParts, "\n")
	bottom := rule + "\n" + foot

	// Pad the body with blank lines so the bottom rule + footer sit on the
	// last terminal row.  Skip when we have no height yet (pre-resize).
	if m.height > 0 {
		target := m.height - lipgloss.Height(header) - lipgloss.Height(bottom)
		if pad := target - lipgloss.Height(body); pad > 0 {
			body += strings.Repeat("\n", pad)
		}
	}

	return header + "\n" + body + "\n" + bottom
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
