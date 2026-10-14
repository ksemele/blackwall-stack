// Package scroll keeps scroll state for fixed-height lists.
package scroll

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// List manages scroll state for a fixed-height list view.
// It does not render anything by itself — callers use Render() to assemble
// the final string from a pre-built header, a slice of row strings, and a
// footer template.
type List struct {
	scroll int // index of the first visible row
	height int // total lines available (0 = no constraint)
	// HeaderRows counts the fixed rows above the data (header + separator).
	// FooterRows counts the fixed rows below the data (filter line + count line).
	HeaderRows int
	FooterRows int
}

// Height returns the height last set with SetSize (0 = unconstrained).
func (s *List) Height() int { return s.height }

// Reset scrolls back to the first row (used when the data or filter changes).
func (s *List) Reset() { s.scroll = 0 }

// SetSize records the available height from a WindowSizeMsg.
func (s *List) SetSize(height int) {
	s.height = height
}

// visibleCount returns how many data rows fit given the current height.
// Returns a large number when height is 0 (unconstrained).
func (s *List) visibleCount(total int) int {
	if s.height <= 0 {
		return total
	}
	return max(s.height-s.HeaderRows-s.FooterRows, 1)
}

// clamp adjusts scroll so it stays in [0, max(0, total-visible)].
func (s *List) clamp(total int) {
	vis := s.visibleCount(total)
	maxScroll := max(total-vis, 0)
	s.scroll = min(s.scroll, maxScroll)
	s.scroll = max(s.scroll, 0)
}

// HandleKey processes scroll keys and returns true if the key was consumed.
// Caller must only invoke this when not in filter mode.
func (s *List) HandleKey(msg tea.KeyMsg, total int) bool {
	vis := s.visibleCount(total)
	half := max(vis/2, 1)
	switch msg.String() {
	case "j", "down":
		s.scroll++
		s.clamp(total)
		return true
	case "k", "up":
		s.scroll--
		s.clamp(total)
		return true
	case "ctrl+d", "pgdown":
		s.scroll += half
		s.clamp(total)
		return true
	case "ctrl+u", "pgup":
		s.scroll -= half
		s.clamp(total)
		return true
	case "g":
		s.scroll = 0
		return true
	case "G":
		s.scroll = total - vis
		s.clamp(total)
		return true
	}
	return false
}

// Indicator returns a scroll indicator string like "1–16 of 100 (shown 5/100)".
// Callers embed this into their footer block so the total line count stays
// within the FooterRows budget.
func (s *List) Indicator(total int) string {
	vis := s.visibleCount(total)
	end := min(s.scroll+vis, total)
	first := s.scroll + 1
	last := end
	if total == 0 {
		first = 0
		last = 0
	}
	return fmt.Sprintf("%d–%d of %d", first, last, total)
}

// Render assembles the final view string from a pre-built header block (all
// fixed lines including the separator), a slice of data row strings, and a
// footer block. The footer block must already contain exactly FooterRows lines
// (including the trailing newline); it may embed Indicator() output.
//
// When height <= 0 (pre-resize) the full list is returned unchanged.
func (s *List) Render(headerBlock string, rows []string, footerBlock string) string {
	if s.height <= 0 {
		// No size yet — render everything.
		var b strings.Builder
		b.WriteString(headerBlock)
		for _, r := range rows {
			b.WriteString(r)
			b.WriteByte('\n')
		}
		b.WriteString(footerBlock)
		return b.String()
	}

	vis := s.visibleCount(len(rows))
	end := min(s.scroll+vis, len(rows))
	slice := rows[s.scroll:end]

	var b strings.Builder
	b.WriteString(headerBlock)
	for _, r := range slice {
		b.WriteString(r)
		b.WriteByte('\n')
	}
	b.WriteString(footerBlock)
	return b.String()
}
