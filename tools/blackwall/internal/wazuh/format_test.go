package wazuh

import (
	"testing"
	"time"
)

func TestFormatTimestamp(t *testing.T) {
	layout := "2006-01-02 15:04"
	cases := []struct {
		name string
		in   time.Time
		want string
	}{
		{"zero", time.Time{}, "—"},
		{"sentinel-future", time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), "—"},
		{"sentinel-past", time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC), "—"},
		{"normal", time.Date(2026, 5, 17, 15, 57, 0, 0, time.UTC), "2026-05-17 15:57"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FormatTimestamp(tc.in, layout); got != tc.want {
				t.Errorf("FormatTimestamp(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
