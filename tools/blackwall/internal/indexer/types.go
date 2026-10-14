// Package indexer queries Wazuh alerts stored in the Wazuh indexer (OpenSearch).
package indexer

import (
	"encoding/json"
	"fmt"
	"time"
)

// AlertQuery selects alerts for SearchAlerts.
type AlertQuery struct {
	MinLevel      int
	ExcludeGroups []string
	AgentID       string    // "" = all
	RuleID        string    // "" = any
	Since         time.Time // zero = now-24h; otherwise timestamp >= Since
	Size          int       // 0 = 200
	NoTimeLimit   bool      // true = no timestamp range at all (Since is ignored)
}

type Alert struct {
	ID       string         `json:"id"`
	Time     time.Time      `json:"-"` // parsed from "timestamp"
	RawTime  string         `json:"timestamp"`
	Rule     Rule           `json:"rule"`
	Agent    AgentRef       `json:"agent"`
	Data     map[string]any `json:"data"`
	FullLog  string         `json:"full_log"`
	Location string         `json:"location"`
}

type Rule struct {
	ID          string   `json:"id"`
	Level       int      `json:"level"`
	Description string   `json:"description"`
	Groups      []string `json:"groups"`
	FiredTimes  int      `json:"firedtimes"`
	MITRE       MITRE    `json:"mitre"`
	PCIDSS      []string `json:"pci_dss"`
	GDPR        []string `json:"gdpr"`
	NIST        []string `json:"nist_800_53"`
	HIPAA       []string `json:"hipaa"`
	TSC         []string `json:"tsc"`
}

type MITRE struct {
	ID        []string `json:"id"`
	Tactic    []string `json:"tactic"`
	Technique []string `json:"technique"`
}

type AgentRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// wazuhTimeLayout matches Wazuh's "2026-10-13T23:58:03.908+0000".
const wazuhTimeLayout = "2006-01-02T15:04:05.999-0700"

// ParseTime accepts Wazuh's alert timestamp and plain RFC3339.
func ParseTime(s string) (time.Time, error) {
	if t, err := time.Parse(wazuhTimeLayout, s); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, fmt.Errorf("unrecognised timestamp %q", s)
}

// UnmarshalJSON fills Time from the raw "timestamp" field.
func (a *Alert) UnmarshalJSON(b []byte) error {
	type plain Alert
	if err := json.Unmarshal(b, (*plain)(a)); err != nil {
		return err
	}
	if a.RawTime != "" {
		// One unparseable timestamp must not fail the whole search: keep
		// RawTime and leave Time zero.
		if t, err := ParseTime(a.RawTime); err == nil {
			a.Time = t
		}
	}
	return nil
}
