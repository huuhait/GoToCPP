//go:build linux

package modget

import (
	"os/exec"
	"syscall"
)

// dieWithTest makes the child (and its children) die with the test process.
func dieWithTest(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
