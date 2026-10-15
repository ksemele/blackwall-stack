package tunnel

import (
	"context"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

// Loopback is where forwarded ports listen.
const Loopback = "127.0.0.1"

// ForContext returns c unchanged and a nil Tunnel when c has no ssh block.
// Otherwise it picks free local ports, rewrites copies of the wazuh/indexer
// sections to dial 127.0.0.1:<local port> (keeping the configured server as
// the TLS server name unless tls.server_name is set) and opens the tunnel. On an open error the rewritten
// context and the Down tunnel are still returned; the caller must Close it.
func ForContext(ctx context.Context, c config.Context) (config.Context, *Tunnel, error) {
	if c.SSH == nil || c.Wazuh == nil {
		return c, nil, nil
	}
	n := 1
	if c.Indexer != nil {
		n = 2
	}
	ports, err := FreePorts(n)
	if err != nil {
		return c, nil, err
	}
	spec := Spec{Host: c.SSH.Host, Command: c.SSH.Command, ConnectTimeout: c.SSH.ConnectTimeout}
	if spec.ConnectTimeout <= 0 {
		spec.ConnectTimeout = config.DefaultSSHConnectTimeout
	}

	w := *c.Wazuh
	spec.Forwards = append(spec.Forwards, Forward{LocalPort: ports[0], RemoteHost: w.Server, RemotePort: w.Port})
	if w.TLS.ServerName == "" { // an explicit tls.server_name wins
		w.TLS.ServerName = w.Server
	}
	w.Server, w.Port = Loopback, ports[0]
	c.Wazuh = &w

	if c.Indexer != nil {
		x := *c.Indexer
		spec.Forwards = append(spec.Forwards, Forward{LocalPort: ports[1], RemoteHost: x.Server, RemotePort: x.Port})
		if x.TLS.ServerName == "" {
			x.TLS.ServerName = x.Server
		}
		x.Server, x.Port = Loopback, ports[1]
		c.Indexer = &x
	}

	t, err := Open(ctx, spec)
	return c, t, err
}
