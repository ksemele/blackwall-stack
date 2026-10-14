package agents

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
)

// The detail model is created on Enter, long after the terminal reported its
// size. Without the remembered size its scroll lists stay unconstrained and a
// long list (300+ packages) pushes the header and tab bar off the screen.
func TestView_DetailInheritsWindowSize(t *testing.T) {
	v := NewView(&fakeClient{}, 0)
	_, _ = v.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	_, _ = v.Update(app.SwitchToDetailMsg{AgentID: "002"})

	if v.detail == nil {
		t.Fatal("detail not opened")
	}
	pk, ok := v.detail.subs[3].(*PackagesModel)
	if !ok {
		t.Fatalf("subs[3] is %T, want *PackagesModel", v.detail.subs[3])
	}
	if pk.sl.Height() <= 0 {
		t.Errorf("packages list height = %d, want > 0 (inherited from the window)", pk.sl.Height())
	}
}

// While a detail tab's filter prompt is open, the app must not treat typed
// letters as global shortcuts.
func TestView_CapturesKeysWhileDetailFilterIsOpen(t *testing.T) {
	v := NewView(&fakeClient{}, 0)
	if v.CapturesKeys() {
		t.Fatal("list view must not capture keys")
	}
	_, _ = v.Update(app.SwitchToDetailMsg{AgentID: "002"})
	v.detail.activeTab = 3
	pk := v.detail.subs[3].(*PackagesModel)
	pk.filtering = true
	if !v.CapturesKeys() {
		t.Error("view must capture keys while the packages filter is open")
	}
}
