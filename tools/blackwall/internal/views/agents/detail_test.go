package agents

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/app"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

func TestDetail_RendersAllFields(t *testing.T) {
	d := NewDetail(&fakeClient{}, nil, "001")
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
	d := NewDetail(&fakeClient{}, nil, "001")
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected cmd on Esc")
	}
	if _, ok := cmd().(app.BackMsg); !ok {
		t.Errorf("msg type = %T, want BackMsg", cmd())
	}
}

func TestDetail_ArrowsCycleTabs(t *testing.T) {
	var m tea.Model = NewDetail(&fakeClient{}, nil, "001")
	right := tea.KeyMsg{Type: tea.KeyRight}
	left := tea.KeyMsg{Type: tea.KeyLeft}

	m, _ = m.Update(right)
	m, _ = m.Update(right)
	if got := m.(*DetailModel).activeTab; got != 2 {
		t.Fatalf("activeTab = %d after → →, want 2", got)
	}
	m, _ = m.Update(left)
	if got := m.(*DetailModel).activeTab; got != 1 {
		t.Fatalf("activeTab = %d after ←, want 1", got)
	}
	for range detailTabs {
		m, _ = m.Update(right)
	}
	if got := m.(*DetailModel).activeTab; got != len(detailTabs)-1 {
		t.Errorf("activeTab = %d, want last tab %d (→ stops at the end)", got, len(detailTabs)-1)
	}
}

func TestDetail_LeftOnOverviewEmitsBack(t *testing.T) {
	d := NewDetail(&fakeClient{}, nil, "001")
	_, cmd := d.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if cmd == nil {
		t.Fatal("expected cmd on ← from Overview")
	}
	if _, ok := cmd().(app.BackMsg); !ok {
		t.Errorf("msg type = %T, want BackMsg", cmd())
	}
}

func TestDetail_HKeySwitchesToHardwareTab(t *testing.T) {
	d := NewDetail(&fakeClient{}, nil, "001")
	next, _ := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	dm := next.(*DetailModel)
	if dm.activeTab != 1 {
		t.Errorf("activeTab = %d, want 1 after pressing 'h'", dm.activeTab)
	}
}

func TestDetail_OKeyReturnsToOverviewTab(t *testing.T) {
	d := NewDetail(&fakeClient{}, nil, "001")
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
	d := NewDetail(&fakeClient{}, nil, "001")
	d.activeTab = 1
	out := d.View()
	if !strings.Contains(out, "Hardware") {
		t.Errorf("expected 'Hardware' in view output:\n%s", out)
	}
}

func TestDetail_HKeyActivatesHardwareSubview(t *testing.T) {
	d := NewDetail(&fakeClient{}, nil, "001")
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
	d := NewDetail(&fakeClient{}, nil, "001")
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

func TestDetail_TKeyActivatesProcessesSubview(t *testing.T) {
	d := NewDetail(&fakeClient{}, nil, "001")
	next, _ := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	dm := next.(*DetailModel)

	if dm.activeTab != 2 {
		t.Errorf("activeTab = %d, want 2 after pressing 't'", dm.activeTab)
	}
	sub, ok := dm.subs[2]
	if !ok {
		t.Fatal("expected subs[2] to be set")
	}
	if _, isProc := sub.(*ProcessesModel); !isProc {
		t.Errorf("subs[2] type = %T, want *ProcessesModel", sub)
	}
}

func TestDetail_KKeyActivatesPackagesSubview(t *testing.T) {
	d := NewDetail(&fakeClient{}, nil, "001")
	next, _ := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	dm := next.(*DetailModel)

	if dm.activeTab != 3 {
		t.Errorf("activeTab = %d, want 3 after pressing 'k'", dm.activeTab)
	}
	sub, ok := dm.subs[3]
	if !ok {
		t.Fatal("expected subs[3] to be set")
	}
	if _, isPkg := sub.(*PackagesModel); !isPkg {
		t.Errorf("subs[3] type = %T, want *PackagesModel", sub)
	}
}

func TestDetail_SKeyActivatesSecuritySubview(t *testing.T) {
	d := NewDetail(&fakeClient{}, nil, "001")
	next, _ := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	dm := next.(*DetailModel)

	if dm.activeTab != 5 {
		t.Errorf("activeTab = %d, want 5 after pressing 's'", dm.activeTab)
	}
	sub, ok := dm.subs[5]
	if !ok {
		t.Fatal("expected subs[5] to be set")
	}
	if _, isSec := sub.(*SecurityModel); !isSec {
		t.Errorf("subs[5] type = %T, want *SecurityModel", sub)
	}
}

func TestDetail_FilteringSubviewCapturesKeys(t *testing.T) {
	d := NewDetail(&fakeClient{}, nil, "001")

	// Switch to processes tab (t) and trigger init.
	next, _ := d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	dm := next.(*DetailModel)

	// Deliver loaded msg so the sub-view is ready.
	next2, _ := dm.Update(ProcessesLoadedMsg{Items: []wazuh.Process{
		{PID: 1, Name: "sshd"},
	}})
	dm2 := next2.(*DetailModel)

	// Press '/' to enter filter mode.
	next3, _ := dm2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	dm3 := next3.(*DetailModel)

	pm := dm3.subs[2].(*ProcessesModel)
	if !pm.filtering {
		t.Fatal("processes sub-view should be in filtering mode")
	}

	// Now press 'h' — this is a tab hotkey, but it must go to the filter instead.
	next4, _ := dm3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	dm4 := next4.(*DetailModel)

	if dm4.activeTab != 2 {
		t.Errorf("activeTab changed to %d; 'h' should have been consumed by filter", dm4.activeTab)
	}
	pm4 := dm4.subs[2].(*ProcessesModel)
	if pm4.filter != "h" {
		t.Errorf("filter = %q, want \"h\"", pm4.filter)
	}
}

func TestDetail_HardwareSubviewReceivesMsg(t *testing.T) {
	d := NewDetail(&fakeClient{}, nil, "001")
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

func TestDetail_ContainerWarning(t *testing.T) {
	d := NewDetail(&fakeClient{}, nil, "001")
	d.SetDetail(wazuh.AgentDetail{Agent: wazuh.Agent{ID: "001", Name: "host-01", Status: "active"}})
	if out := d.View(); strings.Contains(out, "DOCKER") || strings.Contains(out, containerWarning) {
		t.Errorf("plain host shows container warning:\n%s", out)
	}

	d.SetDetail(wazuh.AgentDetail{Agent: wazuh.Agent{
		ID: "001", Name: "host-01", Status: "active", Group: []string{"default", wazuh.ContainerGroup},
	}})
	out := d.View()
	for _, want := range []string{"DOCKER", containerWarning} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}
