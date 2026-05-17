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
	left := strings.Join(parts, "  │  ")

	if version == "" || width <= 0 {
		return left
	}

	verLabel := styles.TabInactive.Render(version)
	leftWidth := lipgloss.Width(left)
	verWidth := lipgloss.Width(verLabel)
	gap := width - leftWidth - verWidth
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + verLabel
}
