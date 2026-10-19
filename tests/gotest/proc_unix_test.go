//go:build unix

package gotest

import (
	"os/exec"
	"syscall"
)

// setGroup puts the command in a process group of its own, so that killGroup ends the test
// binaries that gocpp test starts too.
func setGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killGroup(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
