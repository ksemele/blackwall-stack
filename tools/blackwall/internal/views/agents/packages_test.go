package agents

import (
	"context"
	"errors"
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
