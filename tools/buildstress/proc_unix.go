//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// setupCmd makes a child that dies with the tool: a group of its own, killed as a whole when the
// context ends, and (Linux) killed when the tool dies.
func setupCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	setPdeathsig(cmd.SysProcAttr)
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}

// setNice lowers the priority of the tool and of everything it starts.
func setNice(n int) {
	if n > 0 {
		syscall.Setpriority(syscall.PRIO_PROCESS, 0, n)
	}
}
