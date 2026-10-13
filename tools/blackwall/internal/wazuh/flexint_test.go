package wazuh

import (
	"encoding/json"
	"testing"
)

type pidHolder struct {
	PID FlexInt `json:"pid"`
}

func TestFlexInt_StringShape(t *testing.T) {
	var h pidHolder
	if err := json.Unmarshal([]byte(`{"pid": "1"}`), &h); err != nil {
		t.Fatalf("unmarshal string shape: %v", err)
	}
	if h.PID != 1 {
		t.Errorf("PID = %d, want 1", h.PID)
	}
}

func TestFlexInt_IntShape(t *testing.T) {
	var h pidHolder
	if err := json.Unmarshal([]byte(`{"pid": 2}`), &h); err != nil {
		t.Fatalf("unmarshal int shape: %v", err)
	}
	if h.PID != 2 {
		t.Errorf("PID = %d, want 2", h.PID)
	}
}

func TestFlexInt_NullShape(t *testing.T) {
	var h pidHolder
	if err := json.Unmarshal([]byte(`{"pid": null}`), &h); err != nil {
		t.Fatalf("unmarshal null shape: %v", err)
	}
	if h.PID != 0 {
		t.Errorf("PID = %d, want 0", h.PID)
	}
}

func TestFlexInt_EmptyString(t *testing.T) {
	var h pidHolder
	if err := json.Unmarshal([]byte(`{"pid": ""}`), &h); err != nil {
		t.Fatalf("unmarshal empty string: %v", err)
	}
	if h.PID != 0 {
		t.Errorf("PID = %d, want 0", h.PID)
	}
}

func TestFlexInt_RealAPIShape(t *testing.T) {
	// Mimics the shape returned by Wazuh syscollector:
	// pid is a string, ppid/tgid are ints.
	input := `{"pid":"1","ppid":0,"name":"systemd"}`
	var p Process
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("unmarshal real API shape: %v", err)
	}
	if p.PID != 1 {
		t.Errorf("PID = %d, want 1", p.PID)
	}
	if p.PPID != 0 {
		t.Errorf("PPID = %d, want 0", p.PPID)
	}
	if p.Name != "systemd" {
		t.Errorf("Name = %q, want systemd", p.Name)
	}
}
