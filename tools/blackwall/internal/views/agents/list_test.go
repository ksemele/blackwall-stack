package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/tickcmd"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

type fakeClient struct {
	agents []wazuh.Agent
	called int
}

func (f *fakeClient) ListAgents(_ context.Context) ([]wazuh.Agent, error) {
	f.called++
	return f.agents, nil
}

func (f *fakeClient) GetAgent(_ context.Context, _ string) (wazuh.AgentDetail, error) {
	return wazuh.AgentDetail{}, nil
}

func TestListView_RenderShowsAgents(t *testing.T) {
	fc := &fakeClient{agents: []wazuh.Agent{
		{ID: "000", Name: "master", Status: "active"},
		{ID: "001", Name: "host-01", Status: "disconnected"},
	}}
	v := New(fc, 3*time.Second)
	v.SetAgents(fc.agents)

	out := v.View()
	for _, want := range []string{"000", "master", "active", "001", "disconnected"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestListView_TickFiresFetch(t *testing.T) {
	fc := &fakeClient{}
	v := New(fc, 3*time.Second)

	_, _ = v.Update(tickcmd.TickMsg{})
	if fc.called != 0 {
		t.Errorf("fetch should not be synchronous: called=%d", fc.called)
	}
}

func TestListView_PauseSkipsFetch(t *testing.T) {
	fc := &fakeClient{}
	v := New(fc, 3*time.Second)
	v.SetPaused(true)
	_, _ = v.Update(tickcmd.TickMsg{})
	if fc.called > 0 {
		t.Errorf("paused view kicked off %d fetches", fc.called)
	}
}

func TestListView_EnterEmitsSwitchToDetail(t *testing.T) {
	fc := &fakeClient{agents: []wazuh.Agent{{ID: "001"}}}
	v := New(fc, 3*time.Second)
	v.SetAgents(fc.agents)
	_, cmd := v.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected cmd on Enter")
	}
	msg := cmd()
	sd, ok := msg.(app.SwitchToDetailMsg)
	if !ok {
		t.Fatalf("msg type = %T, want SwitchToDetailMsg", msg)
	}
	if sd.AgentID != "001" {
		t.Errorf("AgentID = %q, want 001", sd.AgentID)
	}
}
