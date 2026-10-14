package agents

import (
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/footer"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

// View wraps the list and detail sub-models, managing the push/pop stack.
// It is the top-level views.View registered with the app as ":agents".
type View struct {
	list   *Model
	detail *DetailModel // non-nil while detail is shown
	// size is the last terminal size seen. The detail model is created on
	// Enter, after the startup WindowSizeMsg, so it must be handed this.
	size tea.WindowSizeMsg

	idx AlertSearcher // nil when the context has no indexer
	// pendingIdx holds a SetIndexer request made off the UI loop (context
	// switch runs in a tea.Cmd goroutine); it is applied on the UI loop at the
	// start of Init/Update.
	mu         sync.Mutex
	pendingIdx *indexerSwap
}

type indexerSwap struct{ idx AlertSearcher }

// SetIndexer swaps the indexer used by the agent card (context switch). Safe
// to call from any goroutine; takes effect on the next Init/Update.
func (v *View) SetIndexer(idx AlertSearcher) {
	v.mu.Lock()
	v.pendingIdx = &indexerSwap{idx: idx}
	v.mu.Unlock()
}

func (v *View) applyPending() {
	v.mu.Lock()
	p := v.pendingIdx
	v.pendingIdx = nil
	v.mu.Unlock()
	if p != nil {
		v.idx = p.idx
		v.detail = nil
	}
}

// NewView returns a View wrapping a new list Model.
func NewView(c Client, refresh time.Duration) *View {
	return &View{list: New(c, refresh)}
}

// ReplaceClient swaps the wazuh client used by the list (context switch).
func (v *View) ReplaceClient(c Client) {
	v.list.client = c
	v.list.agents = nil
	v.list.cursor = 0
	v.list.lastErr = ""
	v.detail = nil
}

func (v *View) Name() string { return ":agents" }

func (v *View) Hints() []footer.Hint {
	if v.detail != nil {
		return v.detail.Hints()
	}
	return v.list.Hints()
}

func (v *View) Init() tea.Cmd {
	v.applyPending()
	v.detail = nil
	return v.list.Init()
}

func (v *View) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	v.applyPending()
	if ws, ok := msg.(tea.WindowSizeMsg); ok {
		v.size = ws
	}

	// Handle detail push.
	if sd, ok := msg.(app.SwitchToDetailMsg); ok {
		v.detail = NewDetail(v.list.client, v.idx, sd.AgentID)
		if v.size.Height > 0 {
			_, _ = v.detail.Update(v.size)
		}
		return v, v.detail.Init()
	}

	// Handle back from detail.
	if _, ok := msg.(app.BackMsg); ok {
		v.detail = nil
		return v, nil
	}

	// Delegate to active sub-view.
	if v.detail != nil {
		next, cmd := v.detail.Update(msg)
		if d, ok := next.(*DetailModel); ok {
			v.detail = d
		}
		return v, cmd
	}

	next, cmd := v.list.Update(msg)
	if l, ok := next.(*Model); ok {
		v.list = l
	}
	return v, cmd
}

func (v *View) View() string {
	if v.detail != nil {
		return v.detail.View()
	}
	return v.list.View()
}

// SetAgents is a convenience pass-through used in tests.
func (v *View) SetAgents(a []wazuh.Agent) { v.list.SetAgents(a) }

// CapturesKeys implements views.KeyCapturer: true while the active detail tab
// is in a modal state (e.g. typing into its filter prompt).
func (v *View) CapturesKeys() bool {
	if v.detail == nil {
		return false
	}
	sub, ok := v.detail.subs[v.detail.activeTab]
	if !ok {
		return false
	}
	kc, ok := sub.(keyCapturer)
	return ok && kc.CapturesKeys()
}
