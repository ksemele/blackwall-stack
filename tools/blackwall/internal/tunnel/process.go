package tunnel

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// process is one running ssh (or prefix command) with its stderr tail.
type process struct {
	cmd    *exec.Cmd
	stderr *tail
	done   chan struct{} // closed once the process has exited and been reaped
	err    error         // exit status; valid after done
	stop   sync.Once
}

// stopGrace is how long a SIGTERM-ed group gets before SIGKILL.
const stopGrace = 2 * time.Second

func startProcess(s Spec) (*process, error) {
	argv := Argv(s)
	cmd := exec.Command(argv[0], argv[1:]...)
	p := &process{cmd: cmd, stderr: &tail{}, done: make(chan struct{})}
	cmd.Stdin = nil // /dev/null
	cmd.Stdout = nil
	cmd.Stderr = p.stderr
	// A grandchild (ssh under sshpass) may keep the stderr pipe open after
	// the direct child is gone; do not let Wait hang on it.
	cmd.WaitDelay = stopGrace
	setGroup(cmd)
	// argv may carry a password (sshpass -p): name only the program.
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start %s: %w", argv[0], err)
	}
	go func() {
		p.err = cmd.Wait()
		// Sweep helpers left in the group (e.g. ssh after sshpass died).
		signalGroup(cmd.Process.Pid, true)
		close(p.done)
	}()
	return p, nil
}

// exited reports whether the process has ended.
func (p *process) exited() bool {
	select {
	case <-p.done:
		return true
	default:
		return false
	}
}

// kill terminates the process group (SIGTERM, then SIGKILL after stopGrace)
// and waits until the child is reaped. Safe to call more than once.
func (p *process) kill() {
	p.stop.Do(func() {
		if p.exited() {
			return
		}
		pid := p.cmd.Process.Pid
		kids := children(pid) // before SIGTERM: afterwards they are reparented
		signalGroup(pid, false)
		select {
		case <-p.done:
		case <-time.After(stopGrace):
			signalGroup(pid, true)
			<-p.done
		}
		// A well-behaved wrapper forwards SIGTERM; give its child a moment,
		// then make sure it is gone.
		for i := 0; i < 20 && anyAlive(kids); i++ {
			time.Sleep(50 * time.Millisecond)
		}
		killStray(kids)
	})
	<-p.done
}

func anyAlive(pids []int) bool {
	for _, p := range pids {
		if alive(p) {
			return true
		}
	}
	return false
}

// exitError describes why the process ended, with the stderr tail.
func (p *process) exitError() error {
	msg := "ssh exited"
	if p.err != nil {
		msg += " (" + p.err.Error() + ")"
	}
	return withTail(errors.New(msg), p.stderr)
}

func withTail(err error, t *tail) error {
	if s := t.String(); s != "" {
		return fmt.Errorf("%w: %s", err, s)
	}
	return err
}

// errAborted is returned when waiting was cancelled (tunnel closed).
var errAborted = errors.New("tunnel closed")

// waitReady polls until every local port accepts a TCP connection. It fails
// when the process exits, when timeout passes (the process is then killed)
// or when abort is closed.
func waitReady(p *process, ports []int, timeout time.Duration, abort <-chan struct{}) error {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(100 * time.Millisecond)
	defer tick.Stop()
	ready := make(map[int]bool, len(ports))
	for {
		for _, port := range ports {
			if ready[port] {
				continue
			}
			c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 200*time.Millisecond)
			if err == nil {
				c.Close()
				ready[port] = true
			}
		}
		if len(ready) == len(ports) {
			return nil
		}
		select {
		case <-p.done:
			return p.exitError()
		case <-abort:
			p.kill()
			return errAborted
		case <-deadline.C:
			p.kill()
			return withTail(fmt.Errorf("forwarded ports not ready after %s", timeout), p.stderr)
		case <-tick.C:
		}
	}
}

// tail keeps the last few KiB written to it (ssh's stderr).
type tail struct {
	mu  sync.Mutex
	buf []byte
}

const tailMax = 4096

func (t *tail) Write(b []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, b...)
	if over := len(t.buf) - tailMax; over > 0 {
		t.buf = t.buf[over:]
	}
	return len(b), nil
}

// String returns the last three non-empty lines joined with " | ".
func (t *tail) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	var lines []string
	for _, l := range strings.Split(string(t.buf), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	return strings.Join(lines, " | ")
}
