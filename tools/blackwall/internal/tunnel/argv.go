// Package tunnel reaches loopback-only managers through the system ssh:
// it runs `ssh -N -L 127.0.0.1:<free port>:<server>:<port> <host>`, waits for
// the forwarded ports, supervises the process (reconnect with backoff) and
// kills its whole process group on Close.
package tunnel

import (
	"fmt"
	"math"
	"net"
	"strconv"
	"strings"
	"time"
)

// Forward is one -L forward: 127.0.0.1:LocalPort -> RemoteHost:RemotePort as
// seen from the ssh host.
type Forward struct {
	LocalPort  int
	RemoteHost string
	RemotePort int
}

// Spec describes one tunnel process.
type Spec struct {
	Host           string   // ssh destination (alias or user@host)
	Command        []string // argv prefix replacing "ssh"; nil means plain ssh
	ConnectTimeout time.Duration
	Forwards       []Forward
}

// Argv builds the full command line. BatchMode=yes (never prompt) is added
// only for plain ssh: a custom prefix such as sshpass needs password auth.
func Argv(s Spec) []string {
	argv := []string{"ssh"}
	if len(s.Command) > 0 {
		argv = append([]string{}, s.Command...)
	}
	argv = append(argv, "-N", "-o", "ExitOnForwardFailure=yes")
	if len(s.Command) == 0 {
		argv = append(argv, "-o", "BatchMode=yes")
	}
	if s.ConnectTimeout > 0 {
		secs := int(math.Ceil(s.ConnectTimeout.Seconds()))
		argv = append(argv, "-o", "ConnectTimeout="+strconv.Itoa(secs))
	}
	// Detect a dead link so the supervisor can reconnect.
	argv = append(argv, "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=3")
	for _, f := range s.Forwards {
		remote := f.RemoteHost
		if strings.Contains(remote, ":") { // IPv6 literal
			remote = "[" + remote + "]"
		}
		argv = append(argv, "-L", fmt.Sprintf("127.0.0.1:%d:%s:%d", f.LocalPort, remote, f.RemotePort))
	}
	return append(argv, s.Host)
}

// FreePorts returns n distinct free TCP ports on 127.0.0.1. Each port is found
// by binding :0; all listeners stay open until every port is picked (so they
// differ) and are then released. Another process may grab a port before ssh
// binds it; ExitOnForwardFailure turns that race into a clear error.
func FreePorts(n int) ([]int, error) {
	ports := make([]int, 0, n)
	var ls []net.Listener
	defer func() {
		for _, l := range ls {
			l.Close()
		}
	}()
	for range n {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("pick free local port: %w", err)
		}
		ls = append(ls, l)
		ports = append(ports, l.Addr().(*net.TCPAddr).Port)
	}
	return ports, nil
}
