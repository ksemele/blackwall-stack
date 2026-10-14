package sessions

import (
	"os"
	"testing"
	"time"
)

func ts(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return t
}

func load(t *testing.T) []Session {
	t.Helper()
	b, err := os.ReadFile("testdata/full_log.txt")
	if err != nil {
		t.Fatal(err)
	}
	return Parse(string(b))
}

func TestParse_AllShapes(t *testing.T) {
	got := load(t)
	if len(got) != 7 {
		t.Fatalf("len = %d, want 7 (6 sessions + 1 raw): %+v", len(got), got)
	}

	active := got[0]
	if active.User != "root" || active.TTY != "pts/1" || active.From != "10.0.0.5" ||
		!active.Active || !active.Login.Equal(ts("2026-10-14T02:40:00Z")) || !active.Logout.IsZero() {
		t.Errorf("active = %+v", active)
	}

	closed := got[1]
	if closed.Active || !closed.Logout.Equal(ts("2026-10-13T23:13:05Z")) || closed.From != "172.28.0.1" {
		t.Errorf("closed = %+v", closed)
	}

	console := got[2]
	if console.User != "akrug" || console.TTY != "tty1" || console.From != "" || console.End != "down" {
		t.Errorf("console/down = %+v", console)
	}
	if got[3].End != "crash" || got[4].End != "gone" {
		t.Errorf("crash/gone = %+v / %+v", got[3], got[4])
	}

	reboot := got[5]
	if !reboot.IsReboot() || !reboot.Active || reboot.TTY != "system boot" {
		t.Errorf("reboot = %+v", reboot)
	}

	if got[6].Raw != "this line is not from last" || got[6].User != "" {
		t.Errorf("raw = %+v", got[6])
	}
}

func TestParse_EmptyAndHeaderOnly(t *testing.T) {
	if got := Parse(""); len(got) != 0 {
		t.Errorf("empty → %+v", got)
	}
	if got := Parse("ossec: output: 'blackwall-sessions':\n\nwtmp begins 2026-10-13T23:09:49+00:00\n"); len(got) != 0 {
		t.Errorf("header only → %+v", got)
	}
}

func TestDuration(t *testing.T) {
	got := load(t)
	now := ts("2026-10-14T02:49:00Z")
	if d := got[0].Duration(now); d != 9*time.Minute {
		t.Errorf("active duration = %v, want 9m", d)
	}
	if d := got[1].Duration(now); d != 4*time.Second {
		t.Errorf("closed duration = %v, want 4s", d)
	}
	if d := got[2].Duration(now); d != 0 {
		t.Errorf("down (no logout time) duration = %v, want 0", d)
	}
}
