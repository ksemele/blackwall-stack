//go:build unix

package tunnel

import (
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

// setGroup puts the child in its own process group so helpers it spawns
// (sshpass -> ssh) are killed with it and terminal signals do not reach it.
func setGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func signalGroup(pid int, kill bool) {
	sig := syscall.SIGTERM
	if kill {
		sig = syscall.SIGKILL
	}
	_ = syscall.Kill(-pid, sig) // ESRCH once the group is gone: fine
}

// children lists the direct children of pid (best effort, via pgrep). A
// wrapper such as sshpass starts ssh in a new session, outside our process
// group, so the group kill alone would miss it.
func children(pid int) []int {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil
	}
	var pids []int
	for _, f := range strings.Fields(string(out)) {
		if n, err := strconv.Atoi(f); err == nil && n > 1 {
			pids = append(pids, n)
		}
	}
	return pids
}

// killStray SIGKILLs pids that are still alive.
func killStray(pids []int) {
	for _, p := range pids {
		if syscall.Kill(p, 0) == nil {
			_ = syscall.Kill(p, syscall.SIGKILL)
		}
	}
}

func alive(pid int) bool { return syscall.Kill(pid, 0) == nil }
