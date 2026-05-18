package agents

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

func TestDetail_RendersAllFields(t *testing.T) {
	d := NewDetail(&fakeClient{}, "001")
	d.SetDetail(wazuh.AgentDetail{Agent: wazuh.Agent{
		ID: "001", Name: "host-01", IP: "10.0.0.5", Status: "active",
		Version: "Wazuh v4.14.5", Manager: "master-00",
	}})

	out := d.View()
	for _, want := range []string{"001", "host-01", "10.0.0.5", "active", "v4.14.5", "master-00"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestDetail_EscEmitsBack(t *testing.T) {
	d := NewDetail(&fakeClient{}, "001")
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected cmd on Esc")
	}
	if _, ok := cmd().(app.BackMsg); !ok {
		t.Errorf("msg type = %T, want BackMsg", cmd())
	}
}

func TestDetail_HKeySwitchesToHardwareTab(t *testing.T) {
	d := NewDetail(&fakeClient{}, "001")
	next, _ := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	dm := next.(*DetailModel)
	if dm.activeTab != 1 {
		t.Errorf("activeTab = %d, want 1 after pressing 'h'", dm.activeTab)
	}
}

func TestDetail_OKeyReturnsToOverviewTab(t *testing.T) {
	d := NewDetail(&fakeClient{}, "001")
	// first switch away
	next, _ := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	dm := next.(*DetailModel)
	// then back to overview
	next2, _ := dm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	dm2 := next2.(*DetailModel)
	if dm2.activeTab != 0 {
		t.Errorf("activeTab = %d, want 0 after pressing 'o'", dm2.activeTab)
	}
}

func TestDetail_TabbarRenderedWithHardwareActive(t *testing.T) {
	d := NewDetail(&fakeClient{}, "001")
	d.activeTab = 1
	out := d.View()
	if !strings.Contains(out, "Hardware") {
		t.Errorf("expected 'Hardware' in view output:\n%s", out)
	}
}

func TestDetail_HKeyActivatesHardwareSubview(t *testing.T) {
	d := NewDetail(&fakeClient{}, "001")
	next, _ := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	dm := next.(*DetailModel)

	if dm.activeTab != 1 {
		t.Errorf("activeTab = %d, want 1", dm.activeTab)
	}
	sub, ok := dm.subs[1]
	if !ok {
		t.Fatal("expected subs[1] to be set")
	}
	if _, isHW := sub.(*HardwareModel); !isHW {
		t.Errorf("subs[1] type = %T, want *HardwareModel", sub)
	}
}

func TestDetail_NKeyActivatesNetworkSubview(t *testing.T) {
	d := NewDetail(&fakeClient{}, "001")
	next, _ := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	dm := next.(*DetailModel)

	if dm.activeTab != 4 {
		t.Errorf("activeTab = %d, want 4", dm.activeTab)
	}
	sub, ok := dm.subs[4]
	if !ok {
		t.Fatal("expected subs[4] to be set")
	}
	if _, isNet := sub.(*NetworkModel); !isNet {
		t.Errorf("subs[4] type = %T, want *NetworkModel", sub)
	}
}

func TestDetail_HardwareSubviewReceivesMsg(t *testing.T) {
	d := NewDetail(&fakeClient{}, "001")
	// Switch to hardware tab.
	next, _ := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	dm := next.(*DetailModel)

	// Deliver a loaded message.
	loaded := HardwareLoadedMsg{
		HW: wazuh.Hardware{CPUName: "Intel Xeon"},
		OS: wazuh.OSInfo{OSName: "Debian"},
	}
	next2, _ := dm.Update(loaded)
	dm2 := next2.(*DetailModel)

	sub := dm2.subs[1].(*HardwareModel)
	if !sub.loaded {
		t.Error("hardware sub-view should be loaded")
	}
	if sub.hw.CPUName != "Intel Xeon" {
		t.Errorf("CPUName = %q, want Intel Xeon", sub.hw.CPUName)
	}
}
