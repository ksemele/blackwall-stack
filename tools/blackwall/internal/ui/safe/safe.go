// Package safe strips terminal control characters from text that comes from
// the indexer or from agents (rule descriptions interpolate log fields, agent
// names are set at enrollment), so it cannot rewrite the user's terminal.
package safe

import "strings"

// Line cleans single-line text: tabs become a space; C0 controls (including
// newline and carriage return), DEL and C1 controls are dropped.
func Line(s string) string { return clean(s, false) }

// Block cleans multi-line text like Line but keeps '\n'.
func Block(s string) string { return clean(s, true) }

func clean(s string, keepNL bool) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteByte(' ')
		case r == '\n' && keepNL:
			b.WriteByte('\n')
		case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f:
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
