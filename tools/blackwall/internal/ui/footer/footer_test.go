package footer

import (
	"strings"
	"testing"
)

func TestRender_ContainsHints(t *testing.T) {
	got := Render([]Hint{{Key: "↵", Desc: "open"}, {Key: "q", Desc: "quit"}})
	if !strings.Contains(got, "↵") || !strings.Contains(got, "open") {
		t.Errorf("missing first hint: %q", got)
	}
	if !strings.Contains(got, "q") || !strings.Contains(got, "quit") {
		t.Errorf("missing second hint: %q", got)
	}
}
