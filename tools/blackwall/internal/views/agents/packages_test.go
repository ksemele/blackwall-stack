package agents

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

type fakePackagesClient struct {
	items []wazuh.Package
	err   error
}

func (f *fakePackagesClient) ListPackages(_ context.Context, _ string) ([]wazuh.Package, error) {
	return f.items, f.err
}

func runPackagesInit(fc *fakePackagesClient) *PackagesModel {
	m := NewPackages(fc, "001")
	cmd := m.Init()
	msg := cmd()
	next, _ := m.Update(msg)
	return next.(*PackagesModel)
}

func TestPackages_HappyPath(t *testing.T) {
	fc := &fakePackagesClient{items: []wazuh.Package{
		{Name: "openssh-server", Version: "1:8.9p1-3", Vendor: "Ubuntu Developers"},
		{Name: "nginx", Version: "1.22.0", Vendor: "Nginx Inc"},
	}}
	m := runPackagesInit(fc)

	out := m.View()
	// Vendor is truncated to 16 chars, check prefix.
	for _, want := range []string{"openssh-server", "1:8.9p1-3", "Ubuntu"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestPackages_SlashEntersFilterMode(t *testing.T) {
	fc := &fakePackagesClient{}
	m := runPackagesInit(fc)

	if m.CapturesKeys() {
		t.Fatal("should not be capturing keys initially")
	}

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	pm := next.(*PackagesModel)

	if !pm.CapturesKeys() {
		t.Error("CapturesKeys() should be true after /")
	}
}

func TestPackages_TypingInFilterMode(t *testing.T) {
	fc := &fakePackagesClient{}
	m := runPackagesInit(fc)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	pm := next.(*PackagesModel)
	next2, _ := pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	pm2 := next2.(*PackagesModel)

	if pm2.filter != "n" {
		t.Errorf("filter = %q, want \"n\"", pm2.filter)
	}
}

func TestPackages_EscClearsFilter(t *testing.T) {
	fc := &fakePackagesClient{}
	m := runPackagesInit(fc)

	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	pm := next.(*PackagesModel)
	next2, _ := pm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	pm2 := next2.(*PackagesModel)
	next3, _ := pm2.Update(tea.KeyMsg{Type: tea.KeyEsc})
	pm3 := next3.(*PackagesModel)

	if pm3.filtering {
		t.Error("filtering should be false after esc")
	}
	if pm3.filter != "" {
		t.Errorf("filter should be empty after esc, got %q", pm3.filter)
	}
}

func TestPackages_FilterByName(t *testing.T) {
	fc := &fakePackagesClient{items: []wazuh.Package{
		{Name: "openssh-server", Vendor: "Ubuntu"},
		{Name: "nginx", Vendor: "Nginx Inc"},
	}}
	m := runPackagesInit(fc)
	m.filter = "ssh"

	out := m.View()
	if !strings.Contains(out, "openssh-server") {
		t.Errorf("expected openssh-server in filtered view:\n%s", out)
	}
	if strings.Contains(out, "nginx") {
		t.Errorf("nginx should be filtered out:\n%s", out)
	}
}

func TestPackages_FilterByVendor(t *testing.T) {
	fc := &fakePackagesClient{items: []wazuh.Package{
		{Name: "openssh-server", Vendor: "Ubuntu Developers"},
		{Name: "nginx", Vendor: "Nginx Inc"},
	}}
	m := runPackagesInit(fc)
	m.filter = "nginx inc"

	out := m.View()
	if !strings.Contains(out, "nginx") {
		t.Errorf("expected nginx in vendor-filtered view:\n%s", out)
	}
	if strings.Contains(out, "openssh") {
		t.Errorf("openssh should be filtered out:\n%s", out)
	}
}

func TestPackages_ErrorState(t *testing.T) {
	fc := &fakePackagesClient{err: errors.New("timeout")}
	m := runPackagesInit(fc)

	out := m.View()
	if !strings.Contains(out, "⚠") {
		t.Errorf("expected warning icon:\n%s", out)
	}
	if !strings.Contains(out, "timeout") {
		t.Errorf("expected error message:\n%s", out)
	}
}

func TestPackages_LoadingState(t *testing.T) {
	m := NewPackages(&fakePackagesClient{}, "001")
	out := m.View()
	if !strings.Contains(out, "loading") {
		t.Errorf("expected loading state:\n%s", out)
	}
}

func TestPackages_ScrollBehavior(t *testing.T) {
	// Build 100 distinct packages.
	pkgs := make([]wazuh.Package, 100)
	for i := range pkgs {
		pkgs[i] = wazuh.Package{
			Name:    fmt.Sprintf("pkg-%03d", i),
			Version: "1.0",
			Vendor:  "TestVendor",
		}
	}

	fc := &fakePackagesClient{items: pkgs}
	m := runPackagesInit(fc)

	// Send a WindowSizeMsg: 80 wide, 20 tall.
	// headerRows=2, footerRows=2 => visible = 20 - 2 - 2 = 16 rows.
	m.SetSize(80, 20)

	out := m.View()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")

	// Total lines must be ≤ 20 (header+separator+visible+footer+indicator).
	if len(lines) > 20 {
		t.Errorf("view has %d lines, want ≤ 20", len(lines))
	}

	// Header NAME must be in the first line.
	if !strings.Contains(lines[0], "NAME") {
		t.Errorf("first line should be header, got: %q", lines[0])
	}

	// Last line is count+indicator: "100/100  1–16 of 100"
	lastLine := lines[len(lines)-1]
	if !strings.Contains(lastLine, "of 100") {
		t.Errorf("last line should contain 'of 100', got: %q", lastLine)
	}
	if !strings.Contains(lastLine, "1–") {
		t.Errorf("last line should contain '1–', got: %q", lastLine)
	}

	// Scroll down 5 times with 'j'; first data row should now be pkg-005.
	for i := 0; i < 5; i++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		m = next.(*PackagesModel)
	}
	out2 := m.View()
	lines2 := strings.Split(out2, "\n")
	// First data row is after header (line[0]) and separator (line[1]).
	firstDataRow := lines2[2]
	if !strings.Contains(firstDataRow, "pkg-005") {
		t.Errorf("after 5×j, first data row should be pkg-005, got: %q", firstDataRow)
	}

	// Jump to bottom with 'G'; last data row should be pkg-099.
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	m = next.(*PackagesModel)
	out3 := m.View()
	if !strings.Contains(out3, "pkg-099") {
		t.Errorf("after G, view should contain pkg-099:\n%s", out3)
	}
	// Last line should still contain 'of 100'.
	lines3 := strings.Split(strings.TrimRight(out3, "\n"), "\n")
	ind3 := lines3[len(lines3)-1]
	if !strings.Contains(ind3, "of 100") {
		t.Errorf("indicator after G should contain 'of 100', got: %q", ind3)
	}
}
