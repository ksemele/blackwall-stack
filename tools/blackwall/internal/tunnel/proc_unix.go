//go:build unix

package tunnel

import (
	"os/exec"
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
