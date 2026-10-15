//go:build !unix

package tunnel

import (
	"os"
	"os/exec"
)

func setGroup(*exec.Cmd) {}

// signalGroup can only reach the direct child here.
func signalGroup(pid int, _ bool) {
	if p, err := os.FindProcess(pid); err == nil {
		_ = p.Kill()
	}
}

func children(int) []int { return nil }

func killStray([]int) {}

func alive(int) bool { return false }
