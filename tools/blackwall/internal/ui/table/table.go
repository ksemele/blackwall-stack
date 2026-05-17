// Package table renders k9s-style tables whose columns stretch to fill the
// available terminal width.
//
// A column has a minimum width (its title or a sensible content floor) and an
// optional Flex flag.  Any extra horizontal space is split evenly across the
// flex columns, so the rendered row always occupies the full width — letting a
// full-row background highlight extend cleanly from the first to the last
// character on the line.
package table

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Gap is the number of spaces between adjacent columns.
const Gap = 2

// Col describes a single table column.
type Col struct {
	Title string
	Min   int  // minimum width; ignored if smaller than len(Title)
	Flex  bool // distribute leftover width across flex columns
}

// Widths returns the rendered width of each column given a total target
// width.  When totalWidth is too small to fit the minimums (plus gaps) the
// minimums are returned unchanged and the row will overflow.
func Widths(cols []Col, totalWidth int) []int {
	n := len(cols)
	widths := make([]int, n)
	base := 0
	flexCount := 0
	for i, c := range cols {
		m := c.Min
		if w := lipgloss.Width(c.Title); w > m {
			m = w
		}
		widths[i] = m
		base += m
		if c.Flex {
			flexCount++
		}
	}
	if n > 1 {
		base += Gap * (n - 1)
	}
	extra := totalWidth - base
	if extra <= 0 || flexCount == 0 {
		return widths
	}
	per := extra / flexCount
	rem := extra % flexCount
	for i, c := range cols {
		if !c.Flex {
			continue
		}
		widths[i] += per
		if rem > 0 {
			widths[i]++
			rem--
		}
	}
	return widths
}

// Header renders the titles of the columns using the given widths.
func Header(cols []Col, widths []int) string {
	values := make([]string, len(cols))
	for i, c := range cols {
		values[i] = c.Title
	}
	return Row(widths, values)
}

// Row joins values, each right-padded to its column width, separated by Gap
// spaces.  The trailing column is also right-padded so the row's printable
// width equals sum(widths) + Gap*(n-1) — required for full-width highlight.
func Row(widths []int, values []string) string {
	n := len(widths)
	parts := make([]string, n)
	for i, w := range widths {
		v := ""
		if i < len(values) {
			v = values[i]
		}
		parts[i] = pad(v, w)
	}
	return strings.Join(parts, strings.Repeat(" ", Gap))
}

func pad(s string, w int) string {
	sw := lipgloss.Width(s)
	if sw >= w {
		return s
	}
	return s + strings.Repeat(" ", w-sw)
}
