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
	if pk.sl.height <= 0 {
		t.Errorf("packages list height = %d, want > 0 (inherited from the window)", pk.sl.height)
	}
}
