package statusbar

import (
	"strings"
	"testing"
	"time"
)

func TestRender_LiveState(t *testing.T) {
	got := Render(State{
		Context: "master-00",
		Wazuh:   "172.28.0.10:55000",
		Refresh: 3 * time.Second,
		Paused:  false,
	})
	for _, want := range []string{"master-00", "172.28.0.10:55000", "live", "3s"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

func TestRender_Paused(t *testing.T) {
	got := Render(State{Context: "x", Wazuh: "h:1", Paused: true})
	if !strings.Contains(got, "paused") {
		t.Errorf("expected paused indicator: %q", got)
	}
}
