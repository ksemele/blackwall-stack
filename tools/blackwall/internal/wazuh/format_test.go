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

func TestFormatLocalUTC(t *testing.T) {
	saved := time.Local
	time.Local = time.FixedZone("EEST", 3*60*60)
	t.Cleanup(func() { time.Local = saved })

	in := time.Date(2026, 10, 12, 6, 58, 55, 0, time.UTC)
	if got, want := FormatLocalUTC(in, "2006-01-02 15:04", "15:04Z"), "2026-10-12 09:58 (06:58Z)"; got != want {
		t.Errorf("short = %q, want %q", got, want)
	}
	if got, want := FormatLocalUTC(in, "2006-01-02 15:04:05 MST", "2006-01-02 15:04:05 UTC"),
		"2026-10-12 09:58:55 EEST (2026-10-12 06:58:55 UTC)"; got != want {
		t.Errorf("long = %q, want %q", got, want)
	}
	if got := FormatLocalUTC(time.Time{}, "15:04", "15:04Z"); got != "—" {
		t.Errorf("zero = %q, want —", got)
	}
}
