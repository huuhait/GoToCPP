//go:build unix

package main

import (
	"os/exec"
	"syscall"
	"time"
)

// setupChild makes the child die with the tool: its own process group, killed as a whole on a timeout, and on
// Linux SIGKILL when the tool dies.
func setupChild(cmd *exec.Cmd) {
	attr := &syscall.SysProcAttr{Setpgid: true}
	setPdeathsig(attr)
	cmd.SysProcAttr = attr
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second
}
