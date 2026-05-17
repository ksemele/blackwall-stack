// Package tabbar renders the top tab strip of registered views.
package tabbar

import (
	"fmt"
	"strings"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
)

// Render returns the tab strip given an ordered list of view names
// and the index of the active view.
func Render(viewNames []string, active int) string {
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
	return strings.Join(parts, "  │  ")
}
