package tunnel

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// The test binary doubles as a fake ssh: with BW_FAKE_SSH set, TestMain runs
// fakeSSH instead of the tests. Tunnels in tests use Command: {os.Args[0]},
// so the real argv building is exercised and no real ssh is needed.
//
// Modes (BW_FAKE_SSH, or the content of the file named by BW_FAKE_MODE_FILE):
//
//	listen  accept on every -L local port until killed
//	fail    print an ssh-like error to stderr and exit 255
//	hang    never listen
//	fork    start a child in listen mode (like sshpass -> ssh), write its
//	        pid to BW_FAKE_PIDFILE, then wait
//	fork-setsid  as fork, but the child runs in a new session (outside the
//	        process group) and SIGTERM is not forwarded to it
func TestMain(m *testing.M) {
	if mode := os.Getenv("BW_FAKE_SSH"); mode != "" {
		fakeSSH(mode)
		return
	}
	os.Exit(m.Run())
}

func fakeSSH(mode string) {
	if f := os.Getenv("BW_FAKE_MODE_FILE"); f != "" {
		b, _ := os.ReadFile(f)
		mode = strings.TrimSpace(string(b))
	}
	switch mode {
	case "fail":
		fmt.Fprintln(os.Stderr, "user@prod-manager: Permission denied (publickey).")
		os.Exit(255)
	case "hang":
		for {
			time.Sleep(time.Hour)
		}
	case "fork", "fork-setsid":
		child := exec.Command(os.Args[0], os.Args[1:]...)
		child.Env = append(os.Environ(), "BW_FAKE_SSH=listen", "BW_FAKE_MODE_FILE=")
		if mode == "fork-setsid" { // like sshpass: ssh leaves our process group
			child.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		}
		if err := child.Start(); err != nil {
			os.Exit(1)
		}
		_ = os.WriteFile(os.Getenv("BW_FAKE_PIDFILE"), []byte(strconv.Itoa(child.Process.Pid)), 0o600)
		_ = child.Wait()
		os.Exit(1)
	case "listen":
		for i, a := range os.Args {
			if a != "-L" || i+1 >= len(os.Args) {
				continue
			}
			// 127.0.0.1:<local>:<host>:<port>
			parts := strings.Split(os.Args[i+1], ":")
			l, err := net.Listen("tcp", "127.0.0.1:"+parts[1])
			if err != nil {
				fmt.Fprintln(os.Stderr, "bind: Address already in use")
				os.Exit(255)
			}
			go func() {
				for {
					c, err := l.Accept()
					if err != nil {
						return
					}
					c.Close()
				}
			}()
		}
		for {
			time.Sleep(time.Hour)
		}
	}
	os.Exit(2)
}

// fakeSpec returns a spec that runs the fake ssh in mode on fresh ports.
func fakeSpec(t *testing.T, mode string, forwards int) Spec {
	t.Helper()
	t.Setenv("BW_FAKE_SSH", mode)
	ports, err := FreePorts(forwards)
	if err != nil {
		t.Fatal(err)
	}
	s := Spec{Host: "prod-manager", Command: []string{os.Args[0]}, ConnectTimeout: 3 * time.Second}
	for _, p := range ports {
		s.Forwards = append(s.Forwards, Forward{LocalPort: p, RemoteHost: "127.0.0.1", RemotePort: 55000})
	}
	return s
}

func accepts(port int) bool {
	c, err := net.DialTimeout("tcp", "127.0.0.1:"+strconv.Itoa(port), 200*time.Millisecond)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

// eventually polls cond for up to 3s.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}
