// Package oneshot implements the `--once` text-mode output:
// print a single agents table to stdout and exit.
package oneshot

import (
	"context"
	"fmt"
	"io"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/wazuh"
)

type Lister interface {
	ListAgents(ctx context.Context) ([]wazuh.Agent, error)
}

func Print(ctx context.Context, out io.Writer, c Lister) error {
	agents, err := c.ListAgents(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "%-4s  %-20s  %-16s  %-12s  %s\n", "ID", "NAME", "IP", "STATUS", "LAST SEEN")
	for _, a := range agents {
		last := wazuh.FormatTimestamp(a.LastKeepAlive, "2006-01-02 15:04")
		fmt.Fprintf(out, "%-4s  %-20s  %-16s  %-12s  %s\n", a.ID, a.Name, a.IP, a.Status, last)
	}
	return nil
}
