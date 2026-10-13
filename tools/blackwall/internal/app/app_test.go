package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/views"
)

// fakeView is a minimal View implementation for testing the router.
type fakeView struct {
	name     string
	lastMsg  tea.Msg
	rendered string
}

func (v *fakeView) Init() tea.Cmd                            { return nil }
func (v *fakeView) Update(msg tea.Msg) (tea.Model, tea.Cmd) { v.lastMsg = msg; return v, nil }
func (v *fakeView) View() string                             { return v.rendered }
func (v *fakeView) Name() string                             { return v.name }
func (v *fakeView) Hints() []footer.Hint                     { return nil }

var _ views.View = (*fakeView)(nil) // compile-time check

func TestApp_SwitchViewMsg(t *testing.T) {
	a := fakeView{name: ":a", rendered: "AAA"}
	b := fakeView{name: ":b", rendered: "BBB"}
	m := New(Deps{
		Views:       []views.View{&a, &b},
		InitialView: ":a",
	})

	got, _ := m.Update(SwitchViewMsg{Name: ":b"})
	if got.(*Model).Active().Name() != ":b" {
		t.Errorf("active = %q, want :b", got.(*Model).Active().Name())
	}
}

func TestApp_TabCyclesForward(t *testing.T) {
	a := fakeView{name: ":a"}
	b := fakeView{name: ":b"}
	m := New(Deps{Views: []views.View{&a, &b}, InitialView: ":a"})

	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	if got.(*Model).Active().Name() != ":b" {
		t.Errorf("after Tab active = %q, want :b", got.(*Model).Active().Name())
	}
}

func TestApp_NumberKeyJumps(t *testing.T) {
	a := fakeView{name: ":a"}
	b := fakeView{name: ":b"}
	c := fakeView{name: ":c"}
	m := New(Deps{Views: []views.View{&a, &b, &c}, InitialView: ":a"})

	got, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if got.(*Model).Active().Name() != ":c" {
		t.Errorf("after '3' active = %q, want :c", got.(*Model).Active().Name())
	}
}

func TestApp_PinsFooterToBottomOnResize(t *testing.T) {
	a := fakeView{name: ":a", rendered: "one\ntwo\n"}
	m := New(Deps{Views: []views.View{&a}, InitialView: ":a"})

	const height = 20
	_, _ = m.Update(tea.WindowSizeMsg{Width: 80, Height: height})

	out := m.View()
	if h := lipgloss.Height(out); h != height {
		t.Fatalf("rendered height = %d, want %d", h, height)
	}
	lines := strings.Split(out, "\n")
	// Last line is the footer (empty here, since fakeView has no hints).
	// Second-to-last must be the horizontal rule separating body from footer.
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[len(lines)-2], "─") {
		t.Errorf("expected divider on line above footer, got %q", lines[len(lines)-2])
	}
}

func TestApp_QuitsOnQ(t *testing.T) {
	a := fakeView{name: ":a"}
	m := New(Deps{Views: []views.View{&a}, InitialView: ":a"})

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd")
	}
	if msg := cmd(); msg == nil {
		t.Error("Quit cmd produced nil msg")
	}
}

// sizedFakeView renders exactly as many lines as the height it was given, the
// way a scroll list fills its space.
type sizedFakeView struct {
	fakeView
	height int
}

func (v *sizedFakeView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		v.height = ws.Height
	}
	return v, nil
}

func (v *sizedFakeView) View() string {
	return strings.TrimSuffix(strings.Repeat("row\n", v.height), "\n")
}

// Views get the body size, not the terminal size: a view that fills the height
// it is given must not push the tab bar and status line off the screen.
func TestApp_ViewsGetBodyHeight(t *testing.T) {
	v := &sizedFakeView{fakeView: fakeView{name: ":a"}}
	m := New(Deps{Views: []views.View{v}, InitialView: ":a"})
	const height = 40
	_, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: height})

	if v.height >= height {
		t.Fatalf("view got height %d, want less than the terminal's %d", v.height, height)
	}
	out := m.View()
	if got := lipgloss.Height(out); got != height {
		t.Errorf("rendered %d lines, want exactly %d", got, height)
	}
	if !strings.HasPrefix(out, strings.Split(m.View(), "\n")[0]) || strings.HasPrefix(out, "row") {
		t.Errorf("first line is body, chrome was pushed off:\n%s", strings.Split(out, "\n")[0])
	}
}
