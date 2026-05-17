package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
