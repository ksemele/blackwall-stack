// Package statusbar renders the per-frame status line.
package statusbar

import (
	"fmt"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/ui/styles"
)

type State struct {
	Context   string
	Wazuh     string
	Tunnel    string // "" for direct contexts; e.g. "prod up"
	TunnelBad bool   // render Tunnel as an error (reconnecting / down)
	Indexer   string
	Refresh   time.Duration
	Paused    bool
}

func Render(s State) string {
	mode := fmt.Sprintf("↻ live (%s)", s.Refresh)
	if s.Paused {
		mode = "⏸ paused"
	}
	ctxBadge := styles.ContextBadge.Render(s.Context)
	body := fmt.Sprintf("context: %s · wazuh: %s", ctxBadge, s.Wazuh)
	if s.Tunnel != "" {
		t := s.Tunnel
		if s.TunnelBad {
			t = styles.Error.Render(t)
		}
		body += " · ssh: " + t
	}
	if s.Indexer != "" {
		body += " · indexer: " + s.Indexer
	}
	body += " · " + mode
	return styles.StatusBar.Render(body)
}
