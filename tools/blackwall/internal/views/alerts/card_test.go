package alerts

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
)

func richAlert() indexer.Alert {
	return indexer.Alert{
		ID:   "1791935883.1688070",
		Time: time.Date(2026, 10, 13, 23, 58, 3, 0, time.UTC),
		Rule: indexer.Rule{
			ID: "17101", Level: 9, Description: "Successful login during non-business hours.",
			Groups: []string{"policy_violation", "login_time"}, FiredTimes: 8,
			MITRE:  indexer.MITRE{ID: []string{"T1078"}, Tactic: []string{"Initial Access"}, Technique: []string{"Valid Accounts"}},
			PCIDSS: []string{"10.2.5"},
		},
		Agent:    indexer.AgentRef{ID: "002", Name: "black-host-01"},
		Data:     map[string]any{"srcip": "172.28.0.1", "dstuser": "root", "win": map[string]any{"eventdata": map[string]any{"user": "x"}}},
		FullLog:  "Oct 13 23:58:03 black-host-01 sshd[6900]: " + strings.Repeat("long ", 60),
		Location: "/var/log/auth.log",
	}
}

func TestCard_ShowsAllSections(t *testing.T) {
	out := newCard(richAlert(), 100, 0).view()
	for _, want := range []string{
		"[9] Successful login during non-business hours.",
		"17101", "policy_violation, login_time", "fired 8×",
		"Initial Access", "Valid Accounts", "T1078",
		"black-host-01 (002)", "/var/log/auth.log",
		"srcip", "172.28.0.1", "win.eventdata.user",
		"sshd[6900]", "PCI DSS", "10.2.5",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestCard_WrapsToWidthAndFitsHeight(t *testing.T) {
	c := newCard(richAlert(), 60, 12)
	out := c.view()
	for _, line := range strings.Split(out, "\n") {
		if w := lipgloss.Width(line); w > 60 {
			t.Errorf("line width %d > 60: %q", w, line)
		}
	}
	if h := lipgloss.Height(out); h > 12 {
		t.Errorf("height %d > 12", h)
	}
	before := out
	c.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if c.view() == before {
		t.Error("G must scroll to the bottom")
	}
}

func TestView_EnterOpensCardEscCloses(t *testing.T) {
	f := &fakeSearcher{results: [][]indexer.Alert{{richAlert()}}}
	v := newTestView(f)
	drain(t, v, v.Init())
	_, _ = v.Update(key("enter"))
	out := v.View()
	if !strings.Contains(out, "Successful login during non-business hours.") || !strings.Contains(out, "17101") {
		t.Fatalf("card not shown:\n%s", out)
	}
	_, _ = v.Update(key("esc"))
	if !strings.Contains(v.View(), "level ≥") {
		t.Errorf("esc must return to the feed:\n%s", v.View())
	}
}

func TestCard_ComplianceIsLastSection(t *testing.T) {
	out := newCard(richAlert(), 100, 0).view()
	if strings.Index(out, "PCI DSS") < strings.Index(out, "Full log") {
		t.Errorf("PCI DSS must come after Full log:\n%s", out)
	}
}

func TestCard_StripsTerminalControls(t *testing.T) {
	a := richAlert()
	a.FullLog = "evil\x1b[2J\x1b]0;pwned\x07line\r\ttab"
	a.Data = map[string]any{"k\x1b[31m": "v\x07al"}
	out := newCard(a, 100, 0).view()
	for _, bad := range []string{"\x1b", "\x07", "\r"} {
		if strings.Contains(out, bad) {
			t.Errorf("output contains %q: %q", bad, out)
		}
	}
	for _, want := range []string{"evil", "line", "tab"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestSanitize(t *testing.T) {
	cases := map[string]string{
		"a\tb":              "a b",
		"x\x00y\x7fz":       "xyz",
		"c1\u0085\u009fend": "c1end",
		"héllo ✓":           "héllo ✓",
	}
	for in, want := range cases {
		if got := sanitize(in); got != want {
			t.Errorf("sanitize(%q) = %q, want %q", in, got, want)
		}
	}
}
