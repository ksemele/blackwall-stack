package contexts

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

func sampleConfig() config.Config {
	return config.Config{
		Current: "master-00",
		Contexts: map[string]config.Context{
			"master-00": {
				Description: "Docker test env",
				Wazuh:       &config.WazuhSubsystem{Server: "172.28.0.10", Port: 55000},
			},
			"prod-eu-1": {
				Description: "Production EU",
				Wazuh:       &config.WazuhSubsystem{Server: "wazuh.eu.example.com", Port: 55000},
			},
		},
	}
}

func TestList_RenderShowsAllContexts(t *testing.T) {
	v := New(sampleConfig())
	out := v.View()
	for _, want := range []string{"master-00", "prod-eu-1", "Docker test env"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestList_EnterEmitsContextChanged(t *testing.T) {
	v := New(sampleConfig())
	want := v.contextAt(0)

	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd from Enter")
	}
	msg := cmd()
	cc, ok := msg.(app.ContextChangedMsg)
	if !ok {
		t.Fatalf("msg type = %T, want ContextChangedMsg", msg)
	}
	if cc.Name != want {
		t.Errorf("ContextChangedMsg.Name = %q, want %q", cc.Name, want)
	}
}

func TestList_DownMovesCursor(t *testing.T) {
	v := New(sampleConfig())
	cursorBefore := v.cursor
	v.Update(tea.KeyMsg{Type: tea.KeyDown})
	if v.cursor == cursorBefore {
		t.Error("cursor did not move on KeyDown")
	}
}
