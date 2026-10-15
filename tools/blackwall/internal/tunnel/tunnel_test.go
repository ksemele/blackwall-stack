package tunnel

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/greengrunt/blackwall/tools/blackwall/internal/config"
)

func TestArgv(t *testing.T) {
	fw := []Forward{{LocalPort: 40001, RemoteHost: "127.0.0.1", RemotePort: 55000}}
	idx := append(fw, Forward{LocalPort: 40002, RemoteHost: "::1", RemotePort: 9200})
	common := []string{"-N", "-o", "ExitOnForwardFailure=yes", "-o", "ControlPath=none"}
	alive := []string{"-o", "ConnectTimeout=10", "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=3"}
	cat := func(parts ...[]string) []string {
		var out []string
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	cases := []struct {
		name string
		spec Spec
		want []string
	}{
		{"plain ssh, wazuh only", Spec{Host: "prod", ConnectTimeout: 10 * time.Second, Forwards: fw},
			cat([]string{"ssh"}, common, []string{"-o", "BatchMode=yes"}, alive,
				[]string{"-L", "127.0.0.1:40001:127.0.0.1:55000", "prod"})},
		{"command prefix, with indexer", Spec{Host: "black-sentinel-03", ConnectTimeout: 9500 * time.Millisecond,
			Command: []string{"sshpass", "-p", "pw", "ssh", "-F", "cfg"}, Forwards: idx},
			cat([]string{"sshpass", "-p", "pw", "ssh", "-F", "cfg"}, common, alive,
				[]string{"-L", "127.0.0.1:40001:127.0.0.1:55000", "-L", "127.0.0.1:40002:[::1]:9200", "black-sentinel-03"})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Argv(c.spec); !reflect.DeepEqual(got, c.want) {
				t.Errorf("Argv =\n %q\nwant\n %q", got, c.want)
			}
		})
	}
}

func TestFreePorts_Distinct(t *testing.T) {
	ports, err := FreePorts(5)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]bool{}
	for _, p := range ports {
		if p <= 0 || seen[p] {
			t.Fatalf("ports = %v, want distinct positive", ports)
		}
		seen[p] = true
		if accepts(p) {
			t.Errorf("port %d still bound after FreePorts", p)
		}
	}
}

func TestOpen_ReadyAndCloseKills(t *testing.T) {
	s := fakeSpec(t, "listen", 2)
	tn, err := Open(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if st := tn.Status(); st.State != Up || st.Err != nil {
		t.Fatalf("status = %+v, want up", st)
	}
	for _, f := range s.Forwards {
		if !accepts(f.LocalPort) {
			t.Errorf("port %d not forwarded", f.LocalPort)
		}
	}
	pid := tn.proc.cmd.Process.Pid
	tn.Close()
	tn.Close() // idempotent
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Errorf("ssh pid %d still exists after Close (kill -0: %v)", pid, err)
	}
	for _, f := range s.Forwards {
		if accepts(f.LocalPort) {
			t.Errorf("port %d still open after Close", f.LocalPort)
		}
	}
}

func TestOpen_SSHFailureShowsStderr(t *testing.T) {
	tn, err := Open(context.Background(), fakeSpec(t, "fail", 1))
	defer tn.Close()
	if err == nil || !strings.Contains(err.Error(), "Permission denied (publickey)") ||
		!strings.Contains(err.Error(), "ssh tunnel to prod-manager") {
		t.Fatalf("err = %v, want ssh stderr with host", err)
	}
	if st := tn.Status(); st.State != Down {
		t.Errorf("state = %v, want down", st.State)
	}
}

func TestOpen_TimeoutKillsProcess(t *testing.T) {
	s := fakeSpec(t, "hang", 1)
	s.ConnectTimeout = 500 * time.Millisecond
	start := time.Now()
	tn, err := Open(context.Background(), s)
	defer tn.Close()
	if err == nil || !strings.Contains(err.Error(), "not ready after 500ms") {
		t.Fatalf("err = %v, want timeout", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("Open took %s, want ~timeout", d)
	}
}

func TestOpen_ContextCancelAborts(t *testing.T) {
	s := fakeSpec(t, "hang", 1)
	s.ConnectTimeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	tn, err := Open(ctx, s)
	defer tn.Close()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want context deadline", err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("Open took %s after cancel", d)
	}
}

// sshpass-style wrappers fork the real ssh; Close must kill it too, also
// when it runs in its own session (outside the process group).
func TestClose_KillsGrandchild(t *testing.T) {
	for _, mode := range []string{"fork", "fork-setsid"} {
		t.Run(mode, func(t *testing.T) {
			s := fakeSpec(t, mode, 1)
			pidFile := filepath.Join(t.TempDir(), "pid")
			t.Setenv("BW_FAKE_PIDFILE", pidFile)
			tn, err := Open(context.Background(), s)
			if err != nil {
				t.Fatal(err)
			}
			b, err := os.ReadFile(pidFile)
			if err != nil {
				t.Fatal(err)
			}
			child, _ := strconv.Atoi(string(b))
			tn.Close()
			eventually(t, "grandchild to exit", func() bool {
				return errors.Is(syscall.Kill(child, 0), syscall.ESRCH)
			})
			if accepts(s.Forwards[0].LocalPort) {
				t.Error("forward still open after Close")
			}
		})
	}
}

func TestReconnect_AfterUnexpectedExit(t *testing.T) {
	s := fakeSpec(t, "listen", 1)
	tn, err := open(context.Background(), s, []time.Duration{10 * time.Millisecond, 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer tn.Close()
	tn.mu.Lock()
	old := tn.proc
	tn.mu.Unlock()
	_ = old.cmd.Process.Kill() // ssh dies on its own
	eventually(t, "reconnect", func() bool {
		tn.mu.Lock()
		defer tn.mu.Unlock()
		return tn.proc != old && tn.status.State == Up
	})
	if !accepts(s.Forwards[0].LocalPort) {
		t.Error("forward not back after reconnect")
	}
}

func TestReconnect_GivesUpThenRetry(t *testing.T) {
	modeFile := filepath.Join(t.TempDir(), "mode")
	write := func(m string) {
		if err := os.WriteFile(modeFile, []byte(m), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("listen")
	s := fakeSpec(t, "listen", 1)
	t.Setenv("BW_FAKE_MODE_FILE", modeFile)
	tn, err := open(context.Background(), s, []time.Duration{10 * time.Millisecond, 10 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer tn.Close()

	write("fail")
	tn.mu.Lock()
	p := tn.proc
	tn.mu.Unlock()
	_ = p.cmd.Process.Kill()
	eventually(t, "down after retries", func() bool { return tn.Status().State == Down })
	st := tn.Status()
	if st.Err == nil || !strings.Contains(st.Err.Error(), "Permission denied") || st.Attempt != 2 {
		t.Errorf("status = %+v, want permission error after 2 attempts", st)
	}

	write("listen")
	tn.Retry()
	eventually(t, "up after manual retry", func() bool { return tn.Status().State == Up })
	if !accepts(s.Forwards[0].LocalPort) {
		t.Error("forward not back after Retry")
	}
}

func TestForContext(t *testing.T) {
	t.Run("no ssh block: unchanged, no tunnel", func(t *testing.T) {
		c := config.Context{Wazuh: &config.WazuhSubsystem{Server: "wazuh.example.ts.net", Port: 55000}}
		got, tn, err := ForContext(context.Background(), c)
		if err != nil || tn != nil || got.Wazuh != c.Wazuh {
			t.Fatalf("got %+v, %v, %v", got, tn, err)
		}
	})
	t.Run("ssh block: rewritten to loopback with server name", func(t *testing.T) {
		t.Setenv("BW_FAKE_SSH", "listen")
		orig := config.Context{
			SSH:     &config.SSHTunnel{Host: "prod", Command: []string{os.Args[0]}, ConnectTimeout: 3 * time.Second},
			Wazuh:   &config.WazuhSubsystem{Server: "10.0.0.5", Port: 55000, TLS: config.TLS{CAFile: "ca.pem"}},
			Indexer: &config.IndexerSubsystem{Server: "127.0.0.1", Port: 9200, TLS: config.TLS{InsecureSkipVerify: true}},
		}
		got, tn, err := ForContext(context.Background(), orig)
		defer tn.Close()
		if err != nil {
			t.Fatal(err)
		}
		w, x := got.Wazuh, got.Indexer
		if w.Server != Loopback || w.TLS.ServerName != "10.0.0.5" || w.TLS.CAFile != "ca.pem" || !accepts(w.Port) {
			t.Errorf("wazuh = %+v", w)
		}
		if x.Server != Loopback || x.TLS.ServerName != "127.0.0.1" || !x.TLS.InsecureSkipVerify || !accepts(x.Port) || x.Port == w.Port {
			t.Errorf("indexer = %+v", x)
		}
		if orig.Wazuh.Server != "10.0.0.5" || orig.Indexer.Port != 9200 {
			t.Error("ForContext mutated the configured context")
		}
		if got.Wazuh.TLS.ServerName != "10.0.0.5" {
			t.Error("default server name must be the configured server")
		}
		fw := tn.spec.Forwards
		if len(fw) != 2 || fw[0].RemoteHost != "10.0.0.5" || fw[0].RemotePort != 55000 || fw[1].RemotePort != 9200 {
			t.Errorf("forwards = %+v", fw)
		}
	})
	t.Run("explicit tls.server_name wins", func(t *testing.T) {
		t.Setenv("BW_FAKE_SSH", "listen")
		orig := config.Context{
			SSH:   &config.SSHTunnel{Host: "prod", Command: []string{os.Args[0]}, ConnectTimeout: 3 * time.Second},
			Wazuh: &config.WazuhSubsystem{Server: "127.0.0.1", Port: 55000, TLS: config.TLS{ServerName: "manager.example.test"}},
		}
		got, tn, err := ForContext(context.Background(), orig)
		defer tn.Close()
		if err != nil {
			t.Fatal(err)
		}
		if got.Wazuh.TLS.ServerName != "manager.example.test" {
			t.Errorf("server name = %q", got.Wazuh.TLS.ServerName)
		}
	})
}
