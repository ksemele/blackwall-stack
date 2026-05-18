package agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

type fakeProcessesClient struct {
	items []wazuh.Process
	err   error
}

func (f *fakeProcessesClient) ListProcesses(_ context.Context, _ string) ([]wazuh.Process, error) {
	return f.items, f.err
}

func runProcessesInit(fc *fakeProcessesClient) *ProcessesModel {
	m := NewProcesses(fc, "001")
	cmd := m.Init()
	msg := cmd()
	next, _ := m.Update(msg)
	return next.(*ProcessesModel)
}

func TestProcesses_HappyPath(t *testing.T) {
	fc := &fakeProcessesClient{items: []wazuh.Process{
		{PID: 42, PPID: 1, Name: "sshd", Cmd: "/usr/sbin/sshd", Euser: "root", Resident: 1024},
		{PID: 99, PPID: 1, Name: "nginx", Cmd: "/usr/sbin/nginx", Euser: "www-data", Resident: 2048},
	}}
	m := runProcessesInit(fc)

	out := m.View()
	for _, want := range []string{"42", "sshd", "root"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestProcesses_SlashEntersFilterMode(t *testing.T) {
	fc := &fakeProcessesClient{}
	m := runProcessesInit(fc)

	if m.CapturesKeys() {
		t.Fatal("should not be capturing keys initially")
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	pm := next.(*ProcessesModel)

	if !pm.CapturesKeys() {
		t.Error("CapturesKeys() should be true after /")
	}
	if pm.filtering != true {
		t.Error("filtering should be true")
	}
}

func TestProcesses_TypingInFilterMode(t *testing.T) {
	fc := &fakeProcessesClient{}
	m := runProcessesInit(fc)

	// enter filter mode
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	pm := next.(*ProcessesModel)

	// type 's'
	next2, _ := pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	pm2 := next2.(*ProcessesModel)

	if pm2.filter != "s" {
		t.Errorf("filter = %q, want \"s\"", pm2.filter)
	}
}

func TestProcesses_EscClearsFilter(t *testing.T) {
	fc := &fakeProcessesClient{}
	m := runProcessesInit(fc)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	pm := next.(*ProcessesModel)
	next2, _ := pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	pm2 := next2.(*ProcessesModel)

	next3, _ := pm2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	pm3 := next3.(*ProcessesModel)

	if pm3.filtering {
		t.Error("filtering should be false after esc")
	}
	if pm3.filter != "" {
		t.Errorf("filter should be empty after esc, got %q", pm3.filter)
	}
}

func TestProcesses_EnterAppliesFilter(t *testing.T) {
	fc := &fakeProcessesClient{}
	m := runProcessesInit(fc)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	pm := next.(*ProcessesModel)
	next2, _ := pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	pm2 := next2.(*ProcessesModel)

	next3, _ := pm2.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm3 := next3.(*ProcessesModel)

	if pm3.filtering {
		t.Error("filtering should be false after enter")
	}
	if pm3.filter != "s" {
		t.Errorf("filter should be preserved, got %q", pm3.filter)
	}
}

func TestProcesses_FilterShowsOnlyMatching(t *testing.T) {
	fc := &fakeProcessesClient{items: []wazuh.Process{
		{PID: 1, Name: "sshd", Cmd: "/usr/sbin/sshd", Euser: "root"},
		{PID: 2, Name: "nginx", Cmd: "/usr/sbin/nginx", Euser: "www-data"},
	}}
	m := runProcessesInit(fc)
	m.filter = "ssh"
	m.filtering = false

	out := m.View()
	if !strings.Contains(out, "sshd") {
		t.Errorf("expected sshd in filtered view:\n%s", out)
	}
	if strings.Contains(out, "nginx") {
		t.Errorf("nginx should be filtered out:\n%s", out)
	}
}

func TestProcesses_ErrorState(t *testing.T) {
	fc := &fakeProcessesClient{err: errors.New("connection refused")}
	m := runProcessesInit(fc)

	out := m.View()
	if !strings.Contains(out, "⚠") {
		t.Errorf("expected warning icon:\n%s", out)
	}
	if !strings.Contains(out, "connection refused") {
		t.Errorf("expected error message:\n%s", out)
	}
}

func TestProcesses_LoadingState(t *testing.T) {
	m := NewProcesses(&fakeProcessesClient{}, "001")
	out := m.View()
	if !strings.Contains(out, "loading") {
		t.Errorf("expected loading state:\n%s", out)
	}
}
