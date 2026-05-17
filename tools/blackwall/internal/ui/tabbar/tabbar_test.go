package tabbar

import (
	"strings"
	"testing"
)

func TestRender_HighlightsActive(t *testing.T) {
	out := Render([]string{":contexts", ":agents"}, 1)
	if !strings.Contains(out, "[1]") || !strings.Contains(out, ":contexts") {
		t.Errorf("missing tab 1: %q", out)
	}
	if !strings.Contains(out, "[2]") || !strings.Contains(out, ":agents") {
		t.Errorf("missing tab 2: %q", out)
	}
}

func TestRender_EmptyList(t *testing.T) {
	if got := Render(nil, 0); got != "" {
		t.Errorf("Render(nil) = %q, want empty", got)
	}
}
