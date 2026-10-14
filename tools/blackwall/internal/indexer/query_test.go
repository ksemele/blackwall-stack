package indexer

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func asJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestBuildSearch_Defaults(t *testing.T) {
	got := asJSON(t, BuildSearch(AlertQuery{MinLevel: 5, ExcludeGroups: []string{"sca"}}))
	for _, want := range []string{
		`"size":200`,
		`{"timestamp":{"order":"desc"}}`,
		`{"range":{"rule.level":{"gte":5}}}`,
		`{"range":{"timestamp":{"gte":"now-24h"}}}`,
		`"must_not":[{"terms":{"rule.groups":["sca"]}}]`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
	if strings.Contains(got, "agent.id") || strings.Contains(got, "rule.id") {
		t.Errorf("unexpected agent/rule term in %s", got)
	}
}

func TestBuildSearch_AllFilters(t *testing.T) {
	since := time.Date(2026, 10, 13, 23, 58, 3, 908_000_000, time.UTC)
	got := asJSON(t, BuildSearch(AlertQuery{AgentID: "002", RuleID: "100100", Since: since, Size: 1}))
	for _, want := range []string{
		`"size":1`,
		`{"term":{"agent.id":"002"}}`,
		`{"term":{"rule.id":"100100"}}`,
		`{"range":{"timestamp":{"gte":"2026-10-13T23:58:03.908Z"}}}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %s", want, got)
		}
	}
	if strings.Contains(got, "must_not") {
		t.Errorf("no excluded groups → no must_not, got %s", got)
	}
}

func TestBuildSearch_NoTimeLimit(t *testing.T) {
	got := asJSON(t, BuildSearch(AlertQuery{RuleID: "100100", NoTimeLimit: true}))
	if strings.Contains(got, `"timestamp":{"gte"`) || strings.Contains(got, "now-24h") {
		t.Errorf("NoTimeLimit must not emit a timestamp range: %s", got)
	}
	if !strings.Contains(got, `{"timestamp":{"order":"desc"}}`) {
		t.Errorf("sort must stay: %s", got)
	}
	got = asJSON(t, BuildSearch(AlertQuery{}))
	if !strings.Contains(got, `"now-24h"`) {
		t.Errorf("zero Since without NoTimeLimit must stay now-24h: %s", got)
	}
}
