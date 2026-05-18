package agents

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

type fakeHardwareClient struct {
	hw     wazuh.Hardware
	os     wazuh.OSInfo
	hwErr  error
	osErr  error
}

func (f *fakeHardwareClient) GetHardware(_ context.Context, _ string) (wazuh.Hardware, error) {
	return f.hw, f.hwErr
}

func (f *fakeHardwareClient) GetOSInfo(_ context.Context, _ string) (wazuh.OSInfo, error) {
	return f.os, f.osErr
}

func TestHardware_ViewAfterLoad(t *testing.T) {
	fc := &fakeHardwareClient{
		hw: wazuh.Hardware{
			CPUName:  "Intel Core i7",
			CPUCores: 8,
			CPUMHz:   3600,
			RAMTotal: 16384,
			RAMFree:  4096,
			RAMUsage: 75,
		},
		os: wazuh.OSInfo{
			OSName:     "Ubuntu",
			OSVersion:  "22.04",
			OSCodename: "jammy",
			Hostname:   "srv-01",
			Release:    "5.15.0-generic",
			Architecture: "x86_64",
			OSPlatform: "ubuntu",
		},
	}
	m := NewHardware(fc, "001")
	cmd := m.Init()
	msg := cmd()
	next, _ := m.Update(msg)
	hm := next.(*HardwareModel)

	out := hm.View()
	for _, want := range []string{"Intel Core i7", "Ubuntu", "22.04", "jammy", "srv-01", "x86_64"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestHardware_ViewOnError(t *testing.T) {
	fc := &fakeHardwareClient{hwErr: errors.New("connection refused")}
	m := NewHardware(fc, "001")
	cmd := m.Init()
	msg := cmd()
	next, _ := m.Update(msg)
	hm := next.(*HardwareModel)

	if hm.err == "" {
		t.Fatal("expected err to be set")
	}
	out := hm.View()
	if !strings.Contains(out, "⚠") {
		t.Errorf("expected warning icon in:\n%s", out)
	}
	if !strings.Contains(out, "connection refused") {
		t.Errorf("expected error message in:\n%s", out)
	}
}

func TestHardware_LoadingBeforeInit(t *testing.T) {
	m := NewHardware(&fakeHardwareClient{}, "001")
	out := m.View()
	if !strings.Contains(out, "loading") {
		t.Errorf("expected loading state in:\n%s", out)
	}
}
