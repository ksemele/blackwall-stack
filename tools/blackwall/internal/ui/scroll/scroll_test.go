package scroll

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func rows(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = "r"
	}
	return out
}

func TestList_RenderFitsHeight(t *testing.T) {
	l := List{HeaderRows: 2, FooterRows: 1}
	l.SetSize(10)
	out := l.Render("h1\nh2\n", rows(100), "f\n")
	if got := strings.Count(out, "\n"); got != 10 {
		t.Errorf("rendered %d lines, want 10", got)
	}
}

func TestList_UnsizedRendersEverything(t *testing.T) {
	var l List
	out := l.Render("", rows(5), "")
	if got := strings.Count(out, "\n"); got != 5 {
		t.Errorf("rendered %d lines, want 5", got)
	}
}

func TestList_KeysAndIndicator(t *testing.T) {
	l := List{HeaderRows: 0, FooterRows: 0}
	l.SetSize(10)
	l.HandleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}, 100)
	if got := l.Indicator(100); got != "91–100 of 100" {
		t.Errorf("after G indicator = %q", got)
	}
	l.Reset()
	if got := l.Indicator(100); got != "1–10 of 100" {
		t.Errorf("after Reset indicator = %q", got)
	}
	if l.Height() != 10 {
		t.Errorf("Height() = %d", l.Height())
	}
}
