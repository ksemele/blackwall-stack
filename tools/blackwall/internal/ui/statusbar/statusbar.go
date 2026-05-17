// Package statusbar renders the per-frame status line.
package statusbar

import (
	"fmt"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
)

type State struct {
	Context string
	Wazuh   string
	Refresh time.Duration
	Paused  bool
}

func Render(s State) string {
	mode := fmt.Sprintf("↻ live (%s)", s.Refresh)
	if s.Paused {
		mode = "⏸ paused"
	}
	body := fmt.Sprintf("context: %s · wazuh: %s · %s", s.Context, s.Wazuh, mode)
	return styles.StatusBar.Render(body)
}
