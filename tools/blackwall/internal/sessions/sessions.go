// Package sessions parses `last --time-format iso -w` output shipped by the
// Wazuh agent (rule 100100) into login sessions.
package sessions

import (
	"strings"
	"time"
)

type Session struct {
	User, TTY, From string
	Login           time.Time
	Logout          time.Time // zero while active or when End is set
	Active          bool      // "still logged in" / "still running"
	End             string    // "down", "crash" or "gone" when there is no logout time
	Raw             string    // set (and nothing else) for lines that could not be parsed
}

// IsReboot reports the pseudo-session `last` prints for each boot.
func (s Session) IsReboot() bool { return s.User == "reboot" }

// Duration is now-login for active sessions, logout-login for closed ones,
// and 0 when the end time is unknown (down/crash/gone) or the line is raw.
func (s Session) Duration(now time.Time) time.Duration {
	switch {
	case s.Raw != "" || s.Login.IsZero():
		return 0
	case s.Active:
		return now.Sub(s.Login)
	case !s.Logout.IsZero():
		return s.Logout.Sub(s.Login)
	}
	return 0
}

// Parse returns one Session per `last` line, in input order. The Wazuh
// command-output header, blank lines and the "wtmp begins" footer are skipped;
// a line that does not look like a `last` record is kept verbatim in Raw.
func Parse(fullLog string) []Session {
	var out []Session
	for _, line := range strings.Split(fullLog, "\n") {
		line = strings.TrimRight(line, " \r")
		switch {
		case strings.TrimSpace(line) == "",
			strings.HasPrefix(line, "ossec: output:"),
			strings.HasPrefix(line, "wtmp begins"):
			continue
		}
		if s, ok := parseLine(line); ok {
			out = append(out, s)
		} else {
			out = append(out, Session{Raw: strings.TrimSpace(line)})
		}
	}
	return out
}

func parseLine(line string) (Session, bool) {
	fields := strings.Fields(line)
	at := -1
	var login time.Time
	for i, f := range fields {
		if t, err := time.Parse(time.RFC3339, f); err == nil {
			at, login = i, t
			break
		}
	}
	if at < 1 {
		return Session{}, false
	}
	s := Session{User: fields[0], Login: login}
	switch who := fields[1:at]; len(who) {
	case 0:
	case 1:
		s.TTY = who[0]
	case 2:
		s.TTY, s.From = who[0], who[1]
	default: // "reboot system boot <kernel>"
		s.TTY, s.From = strings.Join(who[:len(who)-1], " "), who[len(who)-1]
	}

	rest := strings.Join(fields[at+1:], " ")
	switch {
	case strings.HasPrefix(rest, "still logged in"), strings.HasPrefix(rest, "still running"):
		s.Active = true
	case strings.HasPrefix(rest, "gone"):
		s.End = "gone"
	case strings.HasPrefix(rest, "- "):
		end := strings.Fields(strings.TrimPrefix(rest, "- "))
		if len(end) == 0 {
			return Session{}, false
		}
		if t, err := time.Parse(time.RFC3339, end[0]); err == nil {
			s.Logout = t
		} else {
			s.End = end[0] // down, crash
		}
	default:
		return Session{}, false
	}
	return s, true
}
