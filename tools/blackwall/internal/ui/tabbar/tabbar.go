// Package tabbar renders the top tab strip of registered views.
package tabbar

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
)

// Render returns the tab strip given an ordered list of view names,
// the index of the active view, the terminal width, and the program
// version string (e.g. "v0.0.1").  When width <= 0 or version is empty
// the right-aligned version label is omitted.
//
// Each tab label is prefixed with its 1-based numeric index: "[1] Name".
// This is correct for the top-level view switcher (app.go) where numeric
// hotkeys 1/2/… are used.  For views with letter hotkeys use RenderWithKeys.
func Render(viewNames []string, active int, width int, version string) string {
	if len(viewNames) == 0 {
		return ""
	}
	var parts []string
	for i, name := range viewNames {
		label := fmt.Sprintf("[%d] %s", i+1, name)
		if i == active {
			parts = append(parts, styles.TabActive.Render(label))
		} else {
			parts = append(parts, styles.TabInactive.Render(label))
		}
	}
	return renderStrip(parts, width, version)
}

// RenderWithKeys is like Render but uses caller-supplied single-character
// hotkey strings as tab prefixes instead of numeric indices.
//
// keys must be the same length as viewNames; each entry should be an
// upper-case letter, e.g. []string{"O","H","T","K","N","S"}.
// IMPORTANT: keep keys in sync with the tabIndex map in detail.go — if you
// add or reorder tabs you must update both slices together.
func RenderWithKeys(viewNames []string, keys []string, active int, width int, version string) string {
	if len(viewNames) == 0 {
		return ""
	}
	var parts []string
	for i, name := range viewNames {
		key := ""
		if i < len(keys) {
			key = keys[i]
		}
		label := fmt.Sprintf("[%s] %s", key, name)
		if i == active {
			parts = append(parts, styles.TabActive.Render(label))
		} else {
			parts = append(parts, styles.TabInactive.Render(label))
		}
	}
	return renderStrip(parts, width, version)
}

// renderStrip joins tab parts and optionally appends a right-aligned version label.
func renderStrip(parts []string, width int, version string) string {
	left := strings.Join(parts, "  │  ")

	if version == "" || width <= 0 {
		return left
	}

	verLabel := styles.TabInactive.Render(version)
	leftWidth := lipgloss.Width(left)
	verWidth := lipgloss.Width(verLabel)
	gap := max(width-leftWidth-verWidth, 1)
	return left + strings.Repeat(" ", gap) + verLabel
}
