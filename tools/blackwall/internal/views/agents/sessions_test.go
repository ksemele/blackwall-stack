package agents

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

const snapshot = `ossec: output: 'blackwall-sessions':
root     pts/1        10.0.0.5         2026-10-14T02:40:00+00:00   still logged in
alice    pts/0        172.28.0.1       2026-10-13T23:13:01+00:00 - 2026-10-13T23:13:05+00:00  (00:00)
reboot   system boot  7.0.14-orbstack- 2026-10-13T23:09:49+00:00   still running
`

type fakeIdx struct{ queries []indexer.AlertQuery }

func (f *fakeIdx) SearchAlerts(_ context.Context, q indexer.AlertQuery) ([]indexer.Alert, error) {
	f.queries = append(f.queries, q)
	if q.RuleID == "100100" {
		return []indexer.Alert{{RawTime: "x", Time: time.Date(2026, 10, 14, 2, 49, 0, 0, time.UTC), FullLog: snapshot}}, nil
	}
	// Suspicious-login lookup: a level-9 alert for alice 30 s after her login.
	return []indexer.Alert{{
		Time: time.Date(2026, 10, 13, 23, 13, 31, 0, time.UTC),
		Rule: indexer.Rule{Level: 9}, Data: map[string]any{"dstuser": "alice"},
	}}, nil
}

func runCmd(t *testing.T, m *SessionsModel, cmd tea.Cmd) {
	t.Helper()
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			return
		}
		var next subView
		next, cmd = m.Update(msg)
		m = next.(*SessionsModel)
	}
}

func TestSessions_ActiveFirstMarkedHistoryAfter(t *testing.T) {
	f := &fakeIdx{}
	m := NewSessions(f, "002")
	m.now = func() time.Time { return time.Date(2026, 10, 14, 2, 49, 0, 0, time.UTC) }
	runCmd(t, m, m.Init())

	if q := f.queries[0]; q.RuleID != "100100" || q.AgentID != "002" || q.Size != 1 || q.MinLevel != 0 || len(q.ExcludeGroups) != 0 || !q.NoTimeLimit {
		t.Errorf("snapshot query = %+v", q)
	}
	if q := f.queries[1]; q.MinLevel != 7 || len(q.ExcludeGroups) != 1 || q.ExcludeGroups[0] != "sca" {
		t.Errorf("suspicious lookup must exclude sca: %+v", q)
	}
	out := m.View()
	root := strings.Index(out, "root")
	alice := strings.Index(out, "alice")
	if root < 0 || alice < 0 || root > alice {
		t.Fatalf("active root must come before history:\n%s", out)
	}
	for _, want := range []string{"still logged in", "9m", "10.0.0.5", "⚠", "snapshot"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "reboot") {
		t.Errorf("reboot pseudo-sessions must be hidden:\n%s", out)
	}
}

func TestSessions_NoSnapshotAndNoIndexer(t *testing.T) {
	m := NewSessions(nil, "002")
	runCmd(t, m, m.Init())
	if !strings.Contains(m.View(), "indexer not configured") {
		t.Errorf("nil indexer view:\n%s", m.View())
	}

	empty := NewSessions(emptyIdx{}, "002")
	runCmd(t, empty, empty.Init())
	if !strings.Contains(empty.View(), "no session snapshot yet") {
		t.Errorf("empty view:\n%s", empty.View())
	}
}

type emptyIdx struct{}

func (emptyIdx) SearchAlerts(context.Context, indexer.AlertQuery) ([]indexer.Alert, error) {
	return nil, nil
}

func TestDetail_SessionsTabAndAlertsJump(t *testing.T) {
	d := NewDetail(&fakeClient{}, &fakeIdx{}, "002")
	d.SetDetail(wazuhDetail("002", "black-host-01"))
	_, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if d.activeTab != 6 || !strings.Contains(d.View(), "[E] Sessions") {
		t.Fatalf("tab = %d, view:\n%s", d.activeTab, d.View())
	}
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if cmd == nil {
		t.Fatal("l must emit a command")
	}
	got, ok := cmd().(app.ShowAgentAlertsMsg)
	if !ok || got.AgentID != "002" || got.AgentName != "black-host-01" {
		t.Errorf("l emitted %#v", got)
	}
}

func wazuhDetail(id, name string) wazuh.AgentDetail {
	return wazuh.AgentDetail{Agent: wazuh.Agent{ID: id, Name: name, Status: "active"}}
}

type evilIdx struct{}

func (evilIdx) SearchAlerts(_ context.Context, q indexer.AlertQuery) ([]indexer.Alert, error) {
	if q.RuleID == "100100" {
		return []indexer.Alert{{Time: time.Date(2026, 10, 14, 2, 49, 0, 0, time.UTC),
			FullLog: "ossec: output: 'x':\nev\x1b[31mil   pts/9   1.2.3.4\x1b]0;x\x07   2026-10-14T02:40:00+00:00   still logged in\ngarbage \x1b[2J line\n"}}, nil
	}
	return nil, nil
}

func TestSessions_StripsControlsFromCells(t *testing.T) {
	m := NewSessions(evilIdx{}, "002")
	runCmd(t, m, m.Init())
	out := m.View()
	if strings.ContainsAny(out, "\x1b\x07") {
		t.Errorf("control chars in view: %q", out)
	}
	if !strings.Contains(out, "ev[31mil") {
		t.Errorf("session row missing:\n%s", out)
	}
	e := NewSessions(emptyIdx{}, "002")
	e.loaded, e.err = true, "boom\x1b[2J"
	if strings.Contains(e.View(), "\x1b[2J") {
		t.Errorf("error text not sanitized: %q", e.View())
	}
}

func TestDetail_AlertsJumpFallsBackToAgentID(t *testing.T) {
	d := NewDetail(&fakeClient{}, &fakeIdx{}, "002")
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if cmd == nil {
		t.Fatal("l must emit a command")
	}
	got, ok := cmd().(app.ShowAgentAlertsMsg)
	if !ok || got.AgentName != "002" {
		t.Errorf("l emitted %#v, want AgentName=002", got)
	}
}
