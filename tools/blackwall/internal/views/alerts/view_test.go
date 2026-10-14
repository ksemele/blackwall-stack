package alerts

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
)

// fakeSearcher returns scripted results in order (repeating the last one once
// the script runs out, like a feed that has nothing new) and records every query.
type fakeSearcher struct {
	queries []indexer.AlertQuery
	results [][]indexer.Alert
	err     error
}

func (f *fakeSearcher) SearchAlerts(_ context.Context, q indexer.AlertQuery) ([]indexer.Alert, error) {
	f.queries = append(f.queries, q)
	if f.err != nil {
		return nil, f.err
	}
	if len(f.results) == 0 {
		return nil, nil
	}
	r := f.results[0]
	if len(f.results) > 1 {
		f.results = f.results[1:]
	}
	if q.AgentID == "" {
		return r, nil
	}
	var out []indexer.Alert
	for _, a := range r {
		if a.Agent.ID == q.AgentID {
			out = append(out, a)
		}
	}
	return out, nil
}

func alert(id string, sec, level int, agentID, agentName, desc string) indexer.Alert {
	return indexer.Alert{
		ID: id, Time: time.Date(2026, 10, 14, 0, 0, sec, 0, time.UTC),
		Rule:  indexer.Rule{ID: "17101", Level: level, Description: desc},
		Agent: indexer.AgentRef{ID: agentID, Name: agentName},
	}
}

func settings() Settings {
	return Settings{Context: "lab", MinLevel: 5, ExcludeGroups: []string{"sca", "blackwall_sessions"}, Refresh: time.Second}
}

// drain runs cmd and every command nested in a tea.BatchMsg, feeding each
// resulting message back into v. Ticks are disabled in tests (v.tick = nil).
func drain(t *testing.T, v *View, cmd tea.Cmd) {
	t.Helper()
	for steps := 0; cmd != nil && steps < 50; steps++ {
		msg := cmd()
		if b, ok := msg.(tea.BatchMsg); ok {
			for _, c := range b {
				drain(t, v, c)
			}
			return
		}
		if msg == nil {
			return
		}
		_, cmd = v.Update(msg)
	}
}

func newTestView(f *fakeSearcher) *View {
	v := New(f, settings())
	v.tick = func(int) tea.Cmd { return nil }
	_, _ = v.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return v
}

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestInit_FirstLoadUsesDefaults(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{alert("A", 1, 9, "002", "black-host-01", "Successful login during non-business hours.")}}}
	v := newTestView(f)
	drain(t, v, v.Init())

	q := f.queries[0]
	if q.MinLevel != 5 || q.Size != initialSize || !q.Since.IsZero() || len(q.ExcludeGroups) != 2 || q.AgentID != "" {
		t.Errorf("first query = %+v", q)
	}
	out := v.View()
	for _, want := range []string{"level ≥ 5", "sca, blackwall_sessions", "black-host-01", "Successful login", "17101"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestTick_PrependsKeepsCursorCountsNew(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{
		{alert("B", 2, 7, "002", "h1", "b"), alert("A", 1, 7, "002", "h1", "a")},
		{alert("C", 2, 7, "002", "h1", "c"), alert("B", 2, 7, "002", "h1", "b")}, // B again: gte on the same ms
	}}
	v := newTestView(f)
	drain(t, v, v.Init())
	_, _ = v.Update(key("down")) // cursor on A

	drain(t, v, func() tea.Msg { return tickMsg{gen: v.gen} })

	if q := f.queries[1]; !q.Since.Equal(alert("B", 2, 0, "", "", "").Time.Add(-30*time.Second)) || q.Size != 0 {
		t.Errorf("incremental query = %+v, want Since=B.Time-30s and default size", q)
	}
	if got := ids(v.rows); got != "CBA" {
		t.Fatalf("rows = %s, want CBA", got)
	}
	if v.rows[v.cursor].ID != "A" {
		t.Errorf("cursor moved to %s, want it to stay on A", v.rows[v.cursor].ID)
	}
	if v.newCount != 1 || !strings.Contains(v.View(), "+1 new") {
		t.Errorf("newCount = %d, view:\n%s", v.newCount, v.View())
	}
}

func TestTick_StaleGenerationIgnored(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{alert("A", 1, 7, "002", "h1", "a")}}}
	v := newTestView(f)
	drain(t, v, v.Init())
	_, cmd := v.Update(tickMsg{gen: v.gen - 1})
	if cmd != nil || len(f.queries) != 1 {
		t.Errorf("stale tick must neither query nor re-arm (queries=%d)", len(f.queries))
	}
	_, _ = v.Update(loadedMsg{gen: v.gen - 1, alerts: []indexer.Alert{alert("Z", 9, 9, "", "", "z")}})
	if ids(v.rows) != "A" {
		t.Errorf("stale result applied: %s", ids(v.rows))
	}
}

func TestKeys_LevelGroupsAgentRequery(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{
		alert("B", 2, 7, "002", "black-host-01", "b"),
		alert("A", 1, 7, "000", "black-master-00", "a"),
	}}}
	v := newTestView(f)
	drain(t, v, v.Init())

	drain(t, v, func() tea.Msg { return key("+") })
	if q := f.queries[len(f.queries)-1]; q.MinLevel != 6 {
		t.Errorf("after + MinLevel = %d", q.MinLevel)
	}
	for i := 0; i < 10; i++ {
		_, cmd := v.Update(key("-"))
		drain(t, v, cmd)
	}
	if q := f.queries[len(f.queries)-1]; q.MinLevel != 0 {
		t.Errorf("min level must clamp at 0, got %d", q.MinLevel)
	}
	_, cmd := v.Update(key("s"))
	drain(t, v, cmd)
	if q := f.queries[len(f.queries)-1]; len(q.ExcludeGroups) != 0 {
		t.Errorf("after s ExcludeGroups = %v, want none", q.ExcludeGroups)
	}
	// Agents cycle in name order over agents seen in the feed, then back to all.
	for _, want := range []string{"002", "000", ""} {
		_, cmd = v.Update(key("a"))
		drain(t, v, cmd)
		if q := f.queries[len(f.queries)-1]; q.AgentID != want {
			t.Errorf("agent filter = %q, want %q", q.AgentID, want)
		}
	}
}

func TestFilter_TypingCapturesKeysAndDoesNotQuery(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{
		alert("B", 2, 7, "002", "h1", "Successful login"),
		alert("A", 1, 7, "002", "h1", "New user added"),
	}}}
	v := newTestView(f)
	drain(t, v, v.Init())
	n := len(f.queries)

	_, _ = v.Update(key("/"))
	if !v.CapturesKeys() {
		t.Fatal("view must capture keys while the filter prompt is open")
	}
	for _, r := range "user" {
		_, _ = v.Update(key(string(r)))
	}
	_, _ = v.Update(key("enter"))
	if v.CapturesKeys() {
		t.Error("enter must close the prompt")
	}
	out := v.View()
	if !strings.Contains(out, "New user added") || strings.Contains(out, "Successful login") {
		t.Errorf("filter not applied:\n%s", out)
	}
	if len(f.queries) != n {
		t.Errorf("filtering must not query (queries %d → %d)", n, len(f.queries))
	}
}

func TestShowAgentAlertsMsg_FiltersAndReloads(t *testing.T) {
	f := &fakeSearcher{}
	v := newTestView(f)
	_, cmd := v.Update(app.ShowAgentAlertsMsg{AgentID: "002", AgentName: "black-host-01"})
	drain(t, v, cmd)
	if len(f.queries) == 0 || f.queries[len(f.queries)-1].AgentID != "002" {
		t.Fatalf("queries = %+v", f.queries)
	}
	if !strings.Contains(v.View(), "agent: black-host-01") {
		t.Errorf("filter line must name the agent:\n%s", v.View())
	}
}

func TestNilSearcher_ShowsHint(t *testing.T) {
	v := New(nil, settings())
	v.tick = func(int) tea.Cmd { return nil }
	drain(t, v, v.Init()) // must not panic
	if !strings.Contains(v.View(), "indexer not configured for context lab") {
		t.Errorf("view:\n%s", v.View())
	}
}

func TestError_KeepsRowsAndShowsWarning(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{alert("A", 1, 7, "002", "h1", "a")}}}
	v := newTestView(f)
	drain(t, v, v.Init())
	f.err = errors.New("HTTP 401: Unauthorized")
	drain(t, v, func() tea.Msg { return tickMsg{gen: v.gen} })
	out := v.View()
	if !strings.Contains(out, "⚠ indexer: HTTP 401") || ids(v.rows) != "A" {
		t.Errorf("rows %s, view:\n%s", ids(v.rows), out)
	}
}

func TestRender_LongDescriptionFitsWidth(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{alert("A", 1, 7, "002", "h1", strings.Repeat("x", 300))}}}
	v := newTestView(f)
	drain(t, v, v.Init())
	for _, line := range strings.Split(v.View(), "\n") {
		if w := lipgloss.Width(line); w > 120 {
			t.Errorf("line width %d > 120: %q", w, line)
		}
	}
}

func TestRender_FitsHeight(t *testing.T) {
	var many []indexer.Alert
	for i := 0; i < 100; i++ {
		many = append(many, alert(string(rune('a'+i%26))+string(rune('A'+i/26)), 100-i, 7, "002", "h1", "d"))
	}
	f := &fakeSearcher{results: [][]indexer.Alert{many}}
	v := newTestView(f)
	drain(t, v, v.Init())
	if h := lipgloss.Height(v.View()); h > 30 {
		t.Errorf("view height %d > 30", h)
	}
}

func TestTick_FilterKeepsSelectionByID(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{
		{alert("M2", 3, 7, "002", "h1", "new user"), alert("N0", 2, 7, "002", "h1", "other"), alert("M1", 1, 7, "002", "h1", "user gone")},
		{alert("P", 7, 7, "002", "h1", "user added"), alert("n6", 6, 7, "002", "h1", "x"), alert("n5", 5, 7, "002", "h1", "x"), alert("n4", 4, 7, "002", "h1", "x")},
	}}
	v := newTestView(f)
	drain(t, v, v.Init())
	_, _ = v.Update(key("/"))
	for _, r := range "user" {
		_, _ = v.Update(key(string(r)))
	}
	_, _ = v.Update(key("enter"))
	_, _ = v.Update(key("down")) // cursor on M1 (visible row 1)

	drain(t, v, func() tea.Msg { return tickMsg{gen: v.gen} })

	if got := v.visible()[v.cursor].ID; got != "M1" {
		t.Errorf("cursor on %s, want M1", got)
	}
	if v.newCount != 1 {
		t.Errorf("newCount = %d, want 1", v.newCount)
	}
	_, _ = v.Update(key("enter")) // must not panic
	if v.card == nil || v.card.alert.ID != "M1" {
		t.Errorf("card = %+v, want alert M1", v.card)
	}
}

func TestReplace_AppliedOnNextInit(t *testing.T) {
	a := &fakeSearcher{results: [][]indexer.Alert{{alert("A", 1, 7, "002", "h1", "a")}}}
	b := &fakeSearcher{}
	v := newTestView(a)
	drain(t, v, v.Init())
	st := settings()
	st.Context = "other"
	v.Replace(b, st)
	drain(t, v, v.Init())
	if len(b.queries) == 0 || len(a.queries) != 1 {
		t.Errorf("queries a=%d b=%d, want 1 and >=1", len(a.queries), len(b.queries))
	}
	if len(v.rows) != 0 {
		t.Errorf("old rows survived: %s", ids(v.rows))
	}
}

func TestTick_SinceFromNewestNonZeroTime(t *testing.T) {
	bad := alert("X", 0, 7, "002", "h1", "bad")
	bad.Time = time.Time{}
	f := &fakeSearcher{results: [][]indexer.Alert{
		{bad, alert("B", 20, 7, "002", "h1", "b"), alert("A", 10, 7, "002", "h1", "a")},
	}}
	v := newTestView(f)
	drain(t, v, v.Init())
	drain(t, v, func() tea.Msg { return tickMsg{gen: v.gen} })
	want := alert("B", 20, 0, "", "", "").Time.Add(-30 * time.Second)
	if q := f.queries[1]; !q.Since.Equal(want) {
		t.Errorf("Since = %v, want %v", q.Since, want)
	}
}

func TestTick_AllTimesZeroDoesFullLoad(t *testing.T) {
	bad := alert("X", 0, 7, "002", "h1", "bad")
	bad.Time = time.Time{}
	f := &fakeSearcher{results: [][]indexer.Alert{{bad}}}
	v := newTestView(f)
	drain(t, v, v.Init())
	drain(t, v, func() tea.Msg { return tickMsg{gen: v.gen} })
	if q := f.queries[1]; !q.Since.IsZero() || q.Size != initialSize {
		t.Errorf("want a full load, got %+v", q)
	}
}

func TestAgentCycle_SurvivesFilteredLoads(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{
		alert("B", 2, 7, "002", "black-host-01", "b"),
		alert("A", 1, 7, "000", "black-master-00", "a"),
	}}}
	v := newTestView(f)
	drain(t, v, v.Init())
	for _, want := range []string{"002", "000", ""} {
		_, cmd := v.Update(key("a"))
		drain(t, v, cmd)
		if q := f.queries[len(f.queries)-1]; q.AgentID != want {
			t.Fatalf("agent filter = %q, want %q", q.AgentID, want)
		}
	}
}

func TestShowAgentAlertsMsg_ClearsTextFilter(t *testing.T) {
	f := &fakeSearcher{}
	v := newTestView(f)
	v.filter, v.filtering = "stale", true
	_, cmd := v.Update(app.ShowAgentAlertsMsg{AgentID: "002", AgentName: "h"})
	drain(t, v, cmd)
	if v.filter != "" || v.filtering {
		t.Errorf("filter = %q filtering = %v, want cleared", v.filter, v.filtering)
	}
}

func TestRender_StripsControlsFromIndexerText(t *testing.T) {
	a := alert("A", 1, 7, "002", "host\x1b[31m", "desc\x1b]0;x\x07end")
	f := &fakeSearcher{results: [][]indexer.Alert{{a}}, err: nil}
	v := newTestView(f)
	drain(t, v, v.Init())
	v.err = "bad\x1b[2Jbody"
	v.agentID, v.agentName = "002", "n\x1bm"
	out := v.View()
	if strings.ContainsAny(out, "\x1b\x07") {
		t.Errorf("control chars from indexer text in view: %q", out)
	}
	for _, bad := range []string{"host\x1b", "desc\x1b", "bad\x1b", "n\x1b"} {
		if strings.Contains(out, bad) {
			t.Errorf("view contains %q: %q", bad, out)
		}
	}
}
