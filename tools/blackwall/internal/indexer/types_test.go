package indexer

import (
	"testing"
	"time"
)

// Wazuh writes "+0000" (no colon); time.RFC3339 rejects it.
func TestParseTime_WazuhFormat(t *testing.T) {
	got, err := ParseTime("2026-10-13T23:58:03.908+0000")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 10, 13, 23, 58, 3, 908_000_000, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseTime_RFC3339AndGarbage(t *testing.T) {
	if _, err := ParseTime("2026-10-13T23:58:03.908Z"); err != nil {
		t.Errorf("RFC3339: %v", err)
	}
	if _, err := ParseTime("yesterday"); err == nil {
		t.Error("garbage must be an error")
	}
}
