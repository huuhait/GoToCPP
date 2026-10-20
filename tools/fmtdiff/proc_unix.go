//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// inGroup makes cmd a process group of its own that is killed as a whole when the context ends
// (and with the program, on Linux): no compiler or test program outlives the tool.
func inGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	setPdeathsig(cmd.SysProcAttr)
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
