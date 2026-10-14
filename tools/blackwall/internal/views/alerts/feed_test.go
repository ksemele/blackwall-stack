package alerts

import (
	"testing"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/indexer"
)

func al(id string, sec int) indexer.Alert {
	return indexer.Alert{ID: id, Time: time.Date(2026, 10, 14, 0, 0, sec, 0, time.UTC)}
}

func ids(as []indexer.Alert) string {
	s := ""
	for _, a := range as {
		s += a.ID
	}
	return s
}

// Several alerts share one millisecond; the incremental query uses gte, so the
// newest shown alert comes back again and must not be duplicated.
func TestMerge_DedupesByID(t *testing.T) {
	rows := []indexer.Alert{al("B", 2), al("A", 1)}
	got, added := merge(rows, []indexer.Alert{al("C", 2), al("B", 2)}, maxRows)
	if ids(got) != "CBA" || added != 1 {
		t.Errorf("got %s added %d, want CBA 1", ids(got), added)
	}
}

func TestMerge_CapsAtMax(t *testing.T) {
	rows := []indexer.Alert{al("B", 2), al("A", 1)}
	got, added := merge(rows, []indexer.Alert{al("C", 3)}, 2)
	if ids(got) != "CB" || added != 1 {
		t.Errorf("got %s added %d, want CB 1", ids(got), added)
	}
}
