package wazuh

import "time"

// FormatTimestamp renders a Wazuh API timestamp using the given layout.
// Returns "—" for zero values and for Wazuh sentinel timestamps
// (year 9999 marks "never disconnected" on the manager-self agent;
// year 0001 appears on never-seen pending registrations).
func FormatTimestamp(t time.Time, layout string) string {
	if t.IsZero() || t.Year() < 1970 || t.Year() > 9000 {
		return "—"
	}
	return t.Format(layout)
}
