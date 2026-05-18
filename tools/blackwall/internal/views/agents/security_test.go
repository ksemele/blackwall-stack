package agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

type fakeSecurityClient struct {
	fimScan       wazuh.SyscheckScan
	fimScanErr    error
	fimEntries    []wazuh.SyscheckEntry
	fimEntriesErr error
	rcScan        wazuh.RootcheckScan
	rcScanErr     error
	rcFindings    []wazuh.RootcheckFinding
	rcFindingsErr error
	scaPolicies   []wazuh.SCAPolicy
	scaErr        error
}

func (f *fakeSecurityClient) GetSyscheckLastScan(_ context.Context, _ string) (wazuh.SyscheckScan, error) {
	return f.fimScan, f.fimScanErr
}

func (f *fakeSecurityClient) ListSyscheckEntries(_ context.Context, _ string) ([]wazuh.SyscheckEntry, error) {
	return f.fimEntries, f.fimEntriesErr
}

func (f *fakeSecurityClient) GetRootcheckLastScan(_ context.Context, _ string) (wazuh.RootcheckScan, error) {
	return f.rcScan, f.rcScanErr
}

func (f *fakeSecurityClient) ListRootcheckFindings(_ context.Context, _ string) ([]wazuh.RootcheckFinding, error) {
	return f.rcFindings, f.rcFindingsErr
}

func (f *fakeSecurityClient) ListSCAPolicies(_ context.Context, _ string) ([]wazuh.SCAPolicy, error) {
	return f.scaPolicies, f.scaErr
}

func TestSecurity_HappyPath(t *testing.T) {
	fc := &fakeSecurityClient{
		fimScan: wazuh.SyscheckScan{Start: "2026-05-17 23:00:00", End: "2026-05-17 23:00:14"},
		fimEntries: []wazuh.SyscheckEntry{
			{File: "/etc/passwd", Type: "modified", Mtime: "2026-05-17 22:55:10"},
			{File: "/etc/shadow", Type: "modified", Mtime: "2026-05-17 22:55:11"},
		},
		rcScan: wazuh.RootcheckScan{Start: "2026-05-17 12:00:00", End: "2026-05-17 12:01:43"},
		rcFindings: []wazuh.RootcheckFinding{
			{Log: "Default user root detected", Status: "outstanding", DateFirst: "2026-05-01"},
			{Log: "/tmp permissions are world-writable", Status: "solved", DateFirst: "2026-05-02"},
		},
		scaPolicies: []wazuh.SCAPolicy{
			{Name: "CIS Ubuntu 22.04", Pass: 145, TotalChecks: 200, EndScan: "2026-05-17 03:00:00"},
		},
	}

	m := NewSecurity(fc, "001")
	cmd := m.Init()
	msg := cmd()
	next, _ := m.Update(msg)
	sm := next.(*SecurityModel)

	out := sm.View()
	for _, want := range []string{
		"Last scan", "2026-05-17 23:00:00",
		"/etc/passwd", "/etc/shadow",
		"2026-05-17 12:00:00",
		"Default user root detected",
		"CIS Ubuntu 22.04",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}

	// solved finding must not appear as outstanding
	if strings.Contains(out, "/tmp permissions") {
		t.Errorf("solved finding should not appear in outstanding: %s", out)
	}
}

func TestSecurity_EmptyPath(t *testing.T) {
	fc := &fakeSecurityClient{}
	m := NewSecurity(fc, "001")
	cmd := m.Init()
	msg := cmd()
	next, _ := m.Update(msg)
	sm := next.(*SecurityModel)

	out := sm.View()
	if !strings.Contains(out, "(no scan yet)") {
		t.Errorf("expected '(no scan yet)' for empty FIM scan in:\n%s", out)
	}
	if !strings.Contains(out, "Recent changes: (none)") {
		t.Errorf("expected 'Recent changes: (none)' in:\n%s", out)
	}
	if !strings.Contains(out, "Outstanding findings: (none)") {
		t.Errorf("expected 'Outstanding findings: (none)' in:\n%s", out)
	}
	if !strings.Contains(out, "(no policies)") {
		t.Errorf("expected '(no policies)' in:\n%s", out)
	}
}

func TestSecurity_PartialErrors(t *testing.T) {
	// FIM entries fails but everything else works.
	fc := &fakeSecurityClient{
		fimScan:       wazuh.SyscheckScan{Start: "2026-05-17 23:00:00", End: "2026-05-17 23:00:14"},
		fimEntriesErr: errors.New("not found"),
		rcScan:        wazuh.RootcheckScan{Start: "2026-05-17 12:00:00", End: "2026-05-17 12:01:43"},
		scaPolicies: []wazuh.SCAPolicy{
			{Name: "PCI-DSS 4.0", Pass: 60, TotalChecks: 80, EndScan: "2026-05-17 03:00:00"},
		},
	}

	m := NewSecurity(fc, "001")
	cmd := m.Init()
	msg := cmd()
	next, _ := m.Update(msg)
	sm := next.(*SecurityModel)

	// Must not panic and must render the sections we have data for.
	out := sm.View()
	if out == "" {
		t.Fatal("expected non-empty view with partial data")
	}
	if !strings.Contains(out, "PCI-DSS 4.0") {
		t.Errorf("expected SCA policy in view:\n%s", out)
	}
	// FIM scan start should still be present (fimScan succeeded).
	if !strings.Contains(out, "2026-05-17 23:00:00") {
		t.Errorf("expected FIM scan timestamp in view:\n%s", out)
	}
	// FIM entries empty → show none.
	if !strings.Contains(out, "Recent changes: (none)") {
		t.Errorf("expected '(none)' for empty FIM entries:\n%s", out)
	}
}

func TestSecurity_LoadingBeforeInit(t *testing.T) {
	m := NewSecurity(&fakeSecurityClient{}, "001")
	out := m.View()
	if !strings.Contains(out, "loading") {
		t.Errorf("expected loading state in:\n%s", out)
	}
}
