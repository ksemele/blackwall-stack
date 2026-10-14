// Package alerts implements the `:alerts` view: a live feed of Wazuh alerts
// from the indexer, with an alert card on Enter.
package alerts

import (
	"context"
	"sort"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
)

type Searcher interface {
	SearchAlerts(ctx context.Context, q indexer.AlertQuery) ([]indexer.Alert, error)
}

type Settings struct {
	Context       string
	MinLevel      int
	ExcludeGroups []string
	Refresh       time.Duration
}

// SearcherFor builds the indexer client for a context. It returns an untyped
// nil (never a nil *indexer.Client wrapped in the interface) when the context
// has no indexer section, so `searcher == nil` checks work.
func SearcherFor(sub *config.IndexerSubsystem) (Searcher, string, error) {
	if sub == nil {
		return nil, "", nil
	}
	c, err := indexer.New(*sub)
	if err != nil {
		return nil, "", err
	}
	return c, c.Addr(), nil
}

// tickMsg and loadedMsg carry the generation that issued them; a filter
// change or re-activation bumps gen, so older ticks and results are dropped.
// The view uses its own tick type: tickcmd.TickMsg goes to whichever view is
// active and would let another view's timer drive this one.
type tickMsg struct{ gen int }

type loadedMsg struct {
	gen         int
	incremental bool
	alerts      []indexer.Alert
	err         error
}

type View struct {
	searcher Searcher
	st       Settings

	rows      []indexer.Alert // newest first
	cursor    int
	offset    int // first visible row
	minLevel  int
	excludeOn bool
	agentID   string
	agentName string
	// knownAgents (id → name) is refreshed from every load and never shrunk by
	// agent-filtered loads, so `a` can cycle over all agents seen.
	knownAgents map[string]string
	filter      string
	filtering   bool
	paused      bool
	newCount    int
	err         string
	gen         int
	width       int
	height      int
	card        *card // non-nil while the alert card is open (Task 9)

	tick func(gen int) tea.Cmd // replaced in tests

	// pending holds a Replace request made off the UI loop; it is applied at
	// the start of Init/Update.
	mu      sync.Mutex
	pending *replacement
}

type replacement struct {
	s  Searcher
	st Settings
}

func New(s Searcher, st Settings) *View {
	v := &View{}
	v.apply(s, st)
	return v
}

// Replace requests a swap of the searcher and settings (context switch). It is
// safe to call from any goroutine; the swap, which drops all state, happens on
// the next Init or Update.
func (v *View) Replace(s Searcher, st Settings) {
	v.mu.Lock()
	v.pending = &replacement{s: s, st: st}
	v.mu.Unlock()
}

func (v *View) applyPending() {
	v.mu.Lock()
	p := v.pending
	v.pending = nil
	v.mu.Unlock()
	if p != nil {
		v.apply(p.s, p.st)
	}
}

func (v *View) apply(s Searcher, st Settings) {
	if st.Refresh <= 0 {
		st.Refresh = 3 * time.Second
	}
	tick := v.tick
	*v = View{searcher: s, st: st, minLevel: st.MinLevel, excludeOn: true,
		width: v.width, height: v.height, gen: v.gen + 1, tick: tick}
	if v.tick != nil {
		return
	}
	v.tick = func(gen int) tea.Cmd {
		return tea.Tick(v.st.Refresh, func(time.Time) tea.Msg { return tickMsg{gen: gen} })
	}
}

func (v *View) Name() string { return ":alerts" }

func (v *View) CapturesKeys() bool { return v.filtering }

func (v *View) Hints() []footer.Hint {
	if v.card != nil {
		return v.card.hints()
	}
	return []footer.Hint{
		{Key: "↵", Desc: "details"}, {Key: "+/-", Desc: "level"}, {Key: "s", Desc: "groups"},
		{Key: "a", Desc: "agent"}, {Key: "/", Desc: "filter"}, {Key: "p", Desc: "pause"},
	}
}

// Init (re)loads from scratch whenever the view is activated.
func (v *View) Init() tea.Cmd {
	v.applyPending()
	return v.reload()
}

func (v *View) reload() tea.Cmd {
	v.gen++
	v.err = ""
	if v.searcher == nil {
		return nil
	}
	return tea.Batch(v.load(false), v.tick(v.gen))
}

func (v *View) query(incremental bool) indexer.AlertQuery {
	q := indexer.AlertQuery{MinLevel: v.minLevel, AgentID: v.agentID}
	if v.excludeOn {
		q.ExcludeGroups = v.st.ExcludeGroups
	}
	if since := v.newestTime(); incremental && !since.IsZero() {
		// Overlap by 30 s: late-indexed alerts may carry slightly older
		// timestamps; merge de-duplicates by ID.
		q.Since = since.Add(-sinceOverlap)
	} else {
		q.Size = initialSize
	}
	return q
}

// sinceOverlap is how far an incremental query reaches back before the
// newest shown alert.
const sinceOverlap = 30 * time.Second

// newestTime returns the newest non-zero alert time among the rows (alerts
// with an unparseable timestamp have a zero Time).
func (v *View) newestTime() time.Time {
	var t time.Time
	for _, r := range v.rows {
		if r.Time.After(t) {
			t = r.Time
		}
	}
	return t
}

func (v *View) load(incremental bool) tea.Cmd {
	s, q, gen := v.searcher, v.query(incremental), v.gen
	inc := !q.Since.IsZero()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		alerts, err := s.SearchAlerts(ctx, q)
		return loadedMsg{gen: gen, incremental: inc, alerts: alerts, err: err}
	}
}

func (v *View) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	v.applyPending()
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width, v.height = msg.Width, msg.Height
		if v.card != nil {
			v.card.setSize(msg.Width, msg.Height)
		}
		return v, nil

	case app.ShowAgentAlertsMsg:
		v.agentID, v.agentName = msg.AgentID, msg.AgentName
		v.filter, v.filtering = "", false
		v.card = nil
		return v, v.reload()

	case tickMsg:
		if msg.gen != v.gen {
			return v, nil
		}
		if v.paused || v.searcher == nil {
			return v, v.tick(v.gen)
		}
		return v, tea.Batch(v.load(true), v.tick(v.gen))

	case loadedMsg:
		if msg.gen != v.gen {
			return v, nil
		}
		if msg.err != nil {
			v.err = msg.err.Error()
			return v, nil
		}
		v.err = ""
		if v.knownAgents == nil {
			v.knownAgents = map[string]string{}
		}
		for _, a := range msg.alerts {
			if a.Agent.ID != "" {
				v.knownAgents[a.Agent.ID] = a.Agent.Name
			}
		}
		if !msg.incremental {
			v.rows, _ = merge(nil, msg.alerts, maxRows)
			v.cursor, v.offset, v.newCount = 0, 0, 0
			return v, nil
		}
		selID := ""
		if vis := v.visible(); v.cursor > 0 && v.cursor < len(vis) {
			selID = vis[v.cursor].ID
		}
		var fresh []indexer.Alert
		oldRows := v.rows
		v.rows, _ = merge(v.rows, msg.alerts, maxRows)
		if v.cursor > 0 {
			known := make(map[string]bool, len(oldRows))
			for _, r := range oldRows {
				known[r.ID] = true
			}
			for _, a := range v.filtered(msg.alerts) {
				if !known[a.ID] {
					known[a.ID] = true
					fresh = append(fresh, a)
				}
			}
			v.newCount += len(fresh)
			vis := v.visible()
			for i, a := range vis {
				if a.ID == selID {
					v.cursor = i
					break
				}
			}
			v.cursor = max(min(v.cursor, len(vis)-1), 0)
		}
		return v, nil

	case tea.KeyMsg:
		if v.card != nil {
			if msg.String() == "esc" {
				v.card = nil
				return v, nil
			}
			v.card.handleKey(msg)
			return v, nil
		}
		if v.filtering {
			v.filterKey(msg)
			return v, nil
		}
		return v, v.key(msg)
	}
	return v, nil
}

func (v *View) filterKey(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyEnter:
		v.filtering = false
	case tea.KeyEsc:
		v.filtering, v.filter = false, ""
	case tea.KeyBackspace:
		if r := []rune(v.filter); len(r) > 0 {
			v.filter = string(r[:len(r)-1])
		}
	case tea.KeyRunes, tea.KeySpace:
		v.filter += string(msg.Runes)
	}
	v.cursor, v.offset = 0, 0
}

func (v *View) key(msg tea.KeyMsg) tea.Cmd {
	rows := v.visible()
	switch msg.String() {
	case "up", "k":
		if v.cursor > 0 {
			v.cursor--
		}
		if v.cursor == 0 {
			v.newCount = 0
		}
	case "down", "j":
		if v.cursor < len(rows)-1 {
			v.cursor++
		}
	case "enter":
		if len(rows) > 0 {
			v.cursor = max(min(v.cursor, len(rows)-1), 0)
			v.card = newCard(rows[v.cursor], v.width, v.height)
		}
	case "+", "=":
		if v.minLevel < 15 {
			v.minLevel++
			return v.reload()
		}
	case "-":
		if v.minLevel > 0 {
			v.minLevel--
			return v.reload()
		}
	case "s":
		v.excludeOn = !v.excludeOn
		return v.reload()
	case "a":
		v.nextAgent()
		return v.reload()
	case "/":
		v.filtering, v.filter = true, ""
	case "p":
		v.paused = !v.paused
	case "ctrl+r":
		return v.reload()
	}
	return nil
}

// nextAgent cycles over the agents present in the feed, by name, then "all".
func (v *View) nextAgent() {
	names := v.knownAgents
	ids := make([]string, 0, len(names))
	for id := range names {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return names[ids[i]] < names[ids[j]] })
	if v.agentID == "" {
		if len(ids) > 0 {
			v.agentID, v.agentName = ids[0], names[ids[0]]
		}
		return
	}
	for i, id := range ids {
		if id == v.agentID {
			if i+1 < len(ids) {
				v.agentID, v.agentName = ids[i+1], names[ids[i+1]]
			} else {
				v.agentID, v.agentName = "", ""
			}
			return
		}
	}
	v.agentID, v.agentName = "", ""
}
