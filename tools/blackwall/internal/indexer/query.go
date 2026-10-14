package indexer

import "time"

const defaultSize = 200

// BuildSearch turns q into an OpenSearch _search body, newest first.
// Since uses gte, not gt: alerts can share one millisecond, and the caller
// de-duplicates by Alert.ID.
func BuildSearch(q AlertQuery) map[string]any {
	size := q.Size
	if size <= 0 {
		size = defaultSize
	}
	since := any("now-24h")
	if !q.Since.IsZero() {
		since = q.Since.UTC().Format(time.RFC3339Nano)
	}
	filter := []any{
		map[string]any{"range": map[string]any{"rule.level": map[string]any{"gte": q.MinLevel}}},
	}
	if !q.NoTimeLimit {
		filter = append(filter, map[string]any{"range": map[string]any{"timestamp": map[string]any{"gte": since}}})
	}
	if q.AgentID != "" {
		filter = append(filter, map[string]any{"term": map[string]any{"agent.id": q.AgentID}})
	}
	if q.RuleID != "" {
		filter = append(filter, map[string]any{"term": map[string]any{"rule.id": q.RuleID}})
	}
	boolQ := map[string]any{"filter": filter}
	if len(q.ExcludeGroups) > 0 {
		boolQ["must_not"] = []any{map[string]any{"terms": map[string]any{"rule.groups": q.ExcludeGroups}}}
	}
	return map[string]any{
		"size":  size,
		"sort":  []any{map[string]any{"timestamp": map[string]any{"order": "desc"}}},
		"query": map[string]any{"bool": boolQ},
	}
}
