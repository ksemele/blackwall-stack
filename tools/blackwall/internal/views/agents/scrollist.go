package agents

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// scrollist manages scroll state for a fixed-height list view.
// It does not render anything by itself — callers use Render() to assemble
// the final string from a pre-built header, a slice of row strings, and a
// footer template.
type scrollist struct {
	scroll int // index of the first visible row
	height int // total lines available (0 = no constraint)
	// headerRows counts the fixed rows above the data (header + separator).
	// footerRows counts the fixed rows below the data (filter line + count line).
	headerRows int
	footerRows int
}

// SetSize records the available height from a WindowSizeMsg.
func (s *scrollist) SetSize(height int) {
	s.height = height
}

// visibleCount returns how many data rows fit given the current height.
// Returns a large number when height is 0 (unconstrained).
func (s *scrollist) visibleCount(total int) int {
	if s.height <= 0 {
		return total
	}
	v := s.height - s.headerRows - s.footerRows
	if v < 1 {
		v = 1
	}
	return v
}

// clamp adjusts scroll so it stays in [0, max(0, total-visible)].
func (s *scrollist) clamp(total int) {
	vis := s.visibleCount(total)
	maxScroll := total - vis
	if maxScroll < 0 {
		maxScroll = 0
	}
	if s.scroll > maxScroll {
		s.scroll = maxScroll
	}
	if s.scroll < 0 {
		s.scroll = 0
	}
}

// HandleKey processes scroll keys and returns true if the key was consumed.
// Caller must only invoke this when not in filter mode.
func (s *scrollist) HandleKey(msg tea.KeyMsg, total int) bool {
	vis := s.visibleCount(total)
	half := vis / 2
	if half < 1 {
		half = 1
	}
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
// within the footerRows budget.
func (s *scrollist) Indicator(total int) string {
	vis := s.visibleCount(total)
	end := s.scroll + vis
	if end > total {
		end = total
	}
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
// footer block. The footer block must already contain exactly footerRows lines
// (including the trailing newline); it may embed Indicator() output.
//
// When height <= 0 (pre-resize) the full list is returned unchanged.
func (s *scrollist) Render(headerBlock string, rows []string, footerBlock string) string {
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
	end := s.scroll + vis
	if end > len(rows) {
		end = len(rows)
	}
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
