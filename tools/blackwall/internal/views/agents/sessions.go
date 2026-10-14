package agents

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/sessions"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/scroll"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/table"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

// AlertSearcher is the indexer capability the agent card needs; nil when the
// context has no indexer.
type AlertSearcher interface {
	SearchAlerts(ctx context.Context, q indexer.AlertQuery) ([]indexer.Alert, error)
}

// sessionsRuleID is the manager rule that stores `last` snapshots
// (blackwall-eye role wazuh-manager, blackwall_rules.xml).
const sessionsRuleID = "100100"

type sessionsLoadedMsg struct {
	snapshot   time.Time
	rows       []sessions.Session
	suspicious map[int]bool
	none       bool
	err        error
}

type SessionsModel struct {
	idx        AlertSearcher
	agentID    string
	now        func() time.Time
	loaded     bool
	none       bool
	err        string
	snapshot   time.Time
	rows       []sessions.Session // active first, then newest first; raw rows last
	suspicious map[int]bool       // index into rows
	sl         scroll.List
	width      int
}

func NewSessions(idx AlertSearcher, agentID string) *SessionsModel {
	return &SessionsModel{idx: idx, agentID: agentID, now: time.Now, sl: scroll.List{HeaderRows: 4, FooterRows: 1}}
}

func (m *SessionsModel) SetSize(width, height int) { m.width = width; m.sl.SetSize(height) }

func (m *SessionsModel) Hints() []footer.Hint {
	return []footer.Hint{{Key: "j/k", Desc: "scroll"}, {Key: "l", Desc: "alerts"}}
}

func (m *SessionsModel) Init() tea.Cmd {
	if m.idx == nil {
		return nil
	}
	idx, id := m.idx, m.agentID
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		snaps, err := idx.SearchAlerts(ctx, indexer.AlertQuery{RuleID: sessionsRuleID, AgentID: id, Size: 1})
		if err != nil {
			return sessionsLoadedMsg{err: err}
		}
		if len(snaps) == 0 {
			return sessionsLoadedMsg{none: true}
		}
		var rows []sessions.Session
		for _, s := range sessions.Parse(snaps[0].FullLog) {
			if !s.IsReboot() {
				rows = append(rows, s)
			}
		}
		sort.SliceStable(rows, func(i, j int) bool {
			if (rows[i].Raw == "") != (rows[j].Raw == "") {
				return rows[i].Raw == ""
			}
			return rows[i].Active && !rows[j].Active
		})
		return sessionsLoadedMsg{snapshot: snaps[0].Time, rows: rows,
			suspicious: markSuspicious(ctx, idx, id, rows)}
	}
}

// markSuspicious flags logins that coincide (±1 min, same user) with an
// alert of level 7 or higher on the agent, e.g. 17101 "login during
// non-business hours". Lookup failures leave everything unmarked.
func markSuspicious(ctx context.Context, idx AlertSearcher, agentID string, rows []sessions.Session) map[int]bool {
	var oldest time.Time
	for _, s := range rows {
		if s.Raw == "" && (oldest.IsZero() || s.Login.Before(oldest)) {
			oldest = s.Login
		}
	}
	out := map[int]bool{}
	if oldest.IsZero() {
		return out
	}
	alerts, err := idx.SearchAlerts(ctx, indexer.AlertQuery{AgentID: agentID, MinLevel: 7, Since: oldest.Add(-time.Minute)})
	if err != nil {
		return out
	}
	for i, s := range rows {
		for _, a := range alerts {
			user, _ := a.Data["dstuser"].(string)
			d := a.Time.Sub(s.Login)
			if s.Raw == "" && user == s.User && d >= -time.Minute && d <= time.Minute {
				out[i] = true
			}
		}
	}
	return out
}

func (m *SessionsModel) Update(msg tea.Msg) (subView, tea.Cmd) {
	switch msg := msg.(type) {
	case sessionsLoadedMsg:
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err.Error()
			return m, nil
		}
		m.err, m.none = "", msg.none
		m.snapshot, m.rows, m.suspicious = msg.snapshot, msg.rows, msg.suspicious
		m.sl.Reset()
	case tea.KeyMsg:
		m.sl.HandleKey(msg, len(m.rows))
	}
	return m, nil
}

func humanDuration(d time.Duration) string {
	switch {
	case d <= 0:
		return "—"
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
	return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
}

var sessionCols = []table.Col{
	{Title: "USER", Min: 10}, {Title: "TTY", Min: 7}, {Title: "FROM", Min: 16},
	{Title: "LOGIN", Min: 25}, {Title: "LOGOUT", Min: 25}, {Title: "DURATION", Min: 9, Flex: true},
}

func (m *SessionsModel) View() string {
	switch {
	case m.idx == nil:
		return "indexer not configured for this context — Sessions need the Wazuh indexer (see tools/blackwall/README.md)\n"
	case !m.loaded:
		return "loading sessions…\n"
	case m.err != "":
		return styles.Error.Render("⚠ indexer: "+m.err) + "\n"
	case m.none:
		return "no session snapshot yet — appears after the next login/logout (rule 100100)\n"
	}
	const local, utc = "2006-01-02 15:04", "15:04Z"
	widths := table.Widths(sessionCols, m.width)
	header := fmt.Sprintf("Sessions — snapshot %s\n\n", wazuh.FormatLocalUTC(m.snapshot, local, utc)) +
		table.Header(sessionCols, widths) + "\n" + strings.Repeat("─", max(m.width, 40)) + "\n"

	now := m.now()
	rows := make([]string, 0, len(m.rows))
	for i, s := range m.rows {
		if s.Raw != "" {
			rows = append(rows, styles.Dim.Render(s.Raw))
			continue
		}
		user := s.User
		if m.suspicious[i] {
			user = "⚠ " + user
		}
		logout := wazuh.FormatLocalUTC(s.Logout, local, utc)
		switch {
		case s.Active:
			logout = "still logged in"
		case s.End != "":
			logout = s.End
		}
		dur := humanDuration(s.Duration(now))
		if s.Active {
			dur += " ●"
		}
		row := table.Row(widths, []string{user, s.TTY, s.From, wazuh.FormatLocalUTC(s.Login, local, utc), logout, dur})
		if s.Active {
			row = styles.Active.Render(row)
		} else if m.suspicious[i] {
			row = styles.RowDown.Render(row)
		}
		rows = append(rows, row)
	}
	return m.sl.Render(header, rows, m.sl.Indicator(len(rows))+"\n")
}
