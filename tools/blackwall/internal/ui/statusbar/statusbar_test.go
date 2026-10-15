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

func TestRender_IndexerShownOnlyWhenSet(t *testing.T) {
	with := Render(State{Context: "lab", Wazuh: "127.0.0.1:55000", Indexer: "127.0.0.1:9200", Refresh: time.Second})
	if !strings.Contains(with, "indexer: 127.0.0.1:9200") {
		t.Errorf("missing indexer in %q", with)
	}
	without := Render(State{Context: "lab", Wazuh: "127.0.0.1:55000", Refresh: time.Second})
	if strings.Contains(without, "indexer") {
		t.Errorf("unexpected indexer in %q", without)
	}
}

func TestRender_TunnelShownOnlyWhenSet(t *testing.T) {
	if got := Render(State{Context: "x", Wazuh: "h:1"}); strings.Contains(got, "ssh:") {
		t.Errorf("direct context must not show ssh: %q", got)
	}
	got := Render(State{Context: "x", Wazuh: "127.0.0.1:55000", Tunnel: "prod down: Permission denied", TunnelBad: true})
	if !strings.Contains(got, "ssh: ") || !strings.Contains(got, "prod down: Permission denied") {
		t.Errorf("tunnel state missing: %q", got)
	}
}
