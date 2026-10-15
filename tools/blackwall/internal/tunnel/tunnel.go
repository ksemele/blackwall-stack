package tunnel

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// State is the supervisor's view of the ssh process.
type State int

const (
	Up           State = iota // forwards answer
	Reconnecting              // process died, retrying with backoff
	Down                      // gave up (or never came up); Retry starts over
)

func (s State) String() string {
	switch s {
	case Up:
		return "up"
	case Reconnecting:
		return "reconnecting"
	default:
		return "down"
	}
}

// Status is a snapshot for the status bar.
type Status struct {
	Host    string
	State   State
	Attempt int // current reconnect attempt (Reconnecting)
	Max     int // reconnect attempts per round
	Err     error
}

// DefaultBackoff is the delay before each automatic reconnect attempt.
var DefaultBackoff = []time.Duration{time.Second, 3 * time.Second, 10 * time.Second}

// Tunnel supervises one ssh process on fixed local ports. After an
// unexpected exit it restarts ssh on the same ports (so clients built for
// them keep working), up to len(backoff) times; then it stays Down until
// Retry. Close kills the process group and stops supervision.
type Tunnel struct {
	spec    Spec
	ports   []int
	backoff []time.Duration

	mu      sync.Mutex
	proc    *process
	status  Status
	looping bool // a reconnect round is running
	closed  bool

	closeCh chan struct{}
	wg      sync.WaitGroup
}

// Open starts the tunnel and waits until every forward accepts connections
// (or spec.ConnectTimeout passes, or ctx is cancelled). On failure it still
// returns the Tunnel, in state Down with the error, so the caller may keep it
// and Retry later; Close is required either way.
func Open(ctx context.Context, s Spec) (*Tunnel, error) {
	return open(ctx, s, DefaultBackoff)
}

func open(ctx context.Context, s Spec, backoff []time.Duration) (*Tunnel, error) {
	t := &Tunnel{spec: s, backoff: backoff, closeCh: make(chan struct{}),
		status: Status{Host: s.Host, Max: len(backoff)}}
	for _, f := range s.Forwards {
		t.ports = append(t.ports, f.LocalPort)
	}
	// Cancelling ctx or calling Close aborts the initial wait.
	abort := make(chan struct{})
	var once sync.Once
	cancel := func() { once.Do(func() { close(abort) }) }
	stopCtx := context.AfterFunc(ctx, cancel)
	defer stopCtx()
	go func() {
		select {
		case <-t.closeCh:
			cancel()
		case <-abort:
		}
	}()
	defer cancel() // releases the goroutine above
	err := t.connect(abort)
	if err != nil && ctx.Err() != nil {
		err = ctx.Err()
	}
	t.mu.Lock()
	if err != nil {
		t.status.State, t.status.Err = Down, err
	}
	t.mu.Unlock()
	return t, err
}

// connect starts one process and waits for it; on success it becomes the
// current process and is watched for unexpected exits.
func (t *Tunnel) connect(abort <-chan struct{}) error {
	p, err := startProcess(t.spec)
	if err != nil {
		return t.wrap(err)
	}
	if err := waitReady(p, t.ports, t.spec.ConnectTimeout, abort); err != nil {
		return t.wrap(err)
	}
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		p.kill()
		return t.wrap(errAborted)
	}
	t.proc = p
	t.looping = false
	t.status = Status{Host: t.spec.Host, State: Up, Max: len(t.backoff)}
	t.wg.Add(1)
	t.mu.Unlock()
	go t.watch(p)
	return nil
}

func (t *Tunnel) wrap(err error) error {
	return fmt.Errorf("ssh tunnel to %s: %w", t.spec.Host, err)
}

// watch waits for p to exit; an exit that Close did not cause starts a
// reconnect round.
func (t *Tunnel) watch(p *process) {
	defer t.wg.Done()
	select {
	case <-p.done:
	case <-t.closeCh:
		return
	}
	t.mu.Lock()
	if t.closed || t.proc != p {
		t.mu.Unlock()
		return
	}
	t.status.Err = t.wrap(p.exitError())
	t.startRoundLocked()
	t.mu.Unlock()
}

// Retry starts a new reconnect round when the tunnel is Down (manual
// refresh). It returns immediately; a no-op while Up or already retrying.
func (t *Tunnel) Retry() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.looping || t.status.State != Down {
		return
	}
	t.startRoundLocked()
}

func (t *Tunnel) startRoundLocked() {
	t.looping = true
	t.status.State = Reconnecting
	t.status.Attempt = 0
	t.wg.Add(1)
	go t.round()
}

func (t *Tunnel) round() {
	defer t.wg.Done()
	var last error
	for i, d := range t.backoff {
		t.mu.Lock()
		t.status.Attempt = i + 1
		t.mu.Unlock()
		select {
		case <-time.After(d):
		case <-t.closeCh:
			return
		}
		if last = t.connect(t.closeCh); last == nil {
			return // connect cleared looping
		}
		if errors.Is(last, errAborted) {
			return
		}
		t.mu.Lock()
		t.status.Err = last
		t.mu.Unlock()
	}
	t.mu.Lock()
	t.looping = false
	t.status.State = Down
	if last != nil {
		t.status.Err = last
	}
	t.mu.Unlock()
}

// Status returns a snapshot of the tunnel state.
func (t *Tunnel) Status() Status {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.status
}

// Close kills the ssh process group and waits for supervision goroutines.
// Safe to call more than once and on a nil Tunnel.
func (t *Tunnel) Close() {
	if t == nil {
		return
	}
	t.mu.Lock()
	if t.closed {
		t.mu.Unlock()
		return
	}
	t.closed = true
	close(t.closeCh)
	p := t.proc
	t.mu.Unlock()
	if p != nil {
		p.kill()
	}
	t.wg.Wait()
}
