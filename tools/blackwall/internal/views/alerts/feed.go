package alerts

import "github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"

const (
	initialSize = 200
	maxRows     = 500
)

// merge prepends the alerts in fresh (newest first) that rows does not have
// yet, keeps newest-first order and caps the result at max rows. It returns
// the merged rows and how many were added.
func merge(rows, fresh []indexer.Alert, max int) ([]indexer.Alert, int) {
	seen := make(map[string]bool, len(rows))
	for _, r := range rows {
		seen[r.ID] = true
	}
	var add []indexer.Alert
	for _, a := range fresh {
		if !seen[a.ID] {
			seen[a.ID] = true
			add = append(add, a)
		}
	}
	out := append(add, rows...)
	if len(out) > max {
		out = out[:max]
	}
	return out, len(add)
}
