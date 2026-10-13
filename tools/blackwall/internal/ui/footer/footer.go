// Package footer renders the bottom keymap hints.
package footer

import (
	"fmt"
	"strings"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
)

type Hint struct {
	Key  string
	Desc string
}

func Render(hints []Hint) string {
	parts := make([]string, 0, len(hints))
	for _, h := range hints {
		parts = append(parts, fmt.Sprintf("[%s] %s", h.Key, h.Desc))
	}
	return styles.Footer.Render(strings.Join(parts, "  "))
}
