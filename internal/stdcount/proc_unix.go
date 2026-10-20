//go:build unix

package stdcount

import (
	"os/exec"
	"syscall"
)

// setProcessGroup puts the test binary in a process group of its own that dies with this process, so that
// the processes a test starts do not outlive the run.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	setParentDeathSignal(cmd.SysProcAttr)
}

// killGroup kills what the test binary left running.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
