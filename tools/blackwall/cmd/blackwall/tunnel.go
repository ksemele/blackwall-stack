package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
	"github.com/greengrunt/blackwall/tools/blackwall/internal/tunnel"
)

// liveTunnel owns the active context's ssh tunnel (nil for direct contexts).
// Context switches open the new tunnel off the UI loop; begin/end track them
// so Close (on exit) can cancel an open in flight and wait for it, leaving
// no ssh process behind.
type liveTunnel struct {
	mu     sync.Mutex
	t      *tunnel.Tunnel
	staged map[*tunnel.Tunnel]bool // opened by a switch, not yet applied
	closed bool
	wg     sync.WaitGroup

	ctx    context.Context // cancelled by Close
	cancel context.CancelFunc
}

var errShuttingDown = errors.New("shutting down")

func newLiveTunnel() *liveTunnel {
	ctx, cancel := context.WithCancel(context.Background())
	return &liveTunnel{ctx: ctx, cancel: cancel}
}

// begin registers an operation that may create a tunnel; false after Close.
func (l *liveTunnel) begin() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return false
	}
	l.wg.Add(1)
	return true
}

func (l *liveTunnel) end() { l.wg.Done() }

// stage records a tunnel opened by a context switch until the switch is
// applied (swap) or abandoned (discard), so Close also kills it when the
// TUI exits in between.
func (l *liveTunnel) stage(t *tunnel.Tunnel) {
	if t == nil {
		return
	}
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		t.Close()
		return
	}
	if l.staged == nil {
		l.staged = map[*tunnel.Tunnel]bool{}
	}
	l.staged[t] = true
	l.mu.Unlock()
}

// discard closes a staged tunnel whose switch failed.
func (l *liveTunnel) discard(t *tunnel.Tunnel) {
	l.mu.Lock()
	delete(l.staged, t)
	l.mu.Unlock()
	t.Close()
}

// swap installs t as the active tunnel and closes the previous one. After
// Close it closes t instead.
func (l *liveTunnel) swap(t *tunnel.Tunnel) {
	l.mu.Lock()
	delete(l.staged, t)
	if l.closed {
		l.mu.Unlock()
		t.Close()
		return
	}
	old := l.t
	l.t = t
	l.mu.Unlock()
	old.Close()
}

func (l *liveTunnel) get() *tunnel.Tunnel {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.t
}

// Retry asks a down tunnel to reconnect (manual refresh).
func (l *liveTunnel) Retry() {
	if t := l.get(); t != nil {
		t.Retry()
	}
}

// Close kills the active tunnel and waits for opens in flight.
func (l *liveTunnel) Close() {
	l.mu.Lock()
	l.closed = true
	t := l.t
	l.t = nil
	staged := l.staged
	l.staged = nil
	l.mu.Unlock()
	l.cancel()
	t.Close()
	for s := range staged {
		s.Close()
	}
	// Opens still in flight see closed and close their own tunnel.
	l.wg.Wait()
}

// label renders the tunnel state for the status bar; bad marks a
// reconnecting or down tunnel.
func (l *liveTunnel) label() (text string, bad bool) {
	t := l.get()
	if t == nil {
		return "", false
	}
	st := t.Status()
	switch st.State {
	case tunnel.Up:
		return st.Host + " up", false
	case tunnel.Reconnecting:
		return fmt.Sprintf("%s reconnecting %d/%d", st.Host, st.Attempt, st.Max), true
	default:
		text = st.Host + " down"
		if st.Err != nil {
			text += ": " + shortErr(st.Err, st.Host)
		}
		return text + " (ctrl+r retries)", true
	}
}

// shortErr drops the "ssh tunnel to <host>: " prefix (the host is already
// shown) and caps the length for the status line.
func shortErr(err error, host string) string {
	s := strings.TrimPrefix(err.Error(), "ssh tunnel to "+host+": ")
	if r := []rune(s); len(r) > 120 {
		s = string(r[:119]) + "…"
	}
	return s
}

// wazuhAddr / indexerAddr show the configured endpoint (as seen from the
// ssh host for tunnelled contexts), not the local forward.
func wazuhAddr(c config.Context) string {
	return c.Wazuh.Server + ":" + strconv.Itoa(c.Wazuh.Port)
}

func indexerAddr(c config.Context) string {
	if c.Indexer == nil {
		return ""
	}
	return c.Indexer.Server + ":" + strconv.Itoa(c.Indexer.Port)
}
