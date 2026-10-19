package bench

import (
	"os/exec"
	"syscall"
)

// setProcAttr makes the server a process group of its own that dies with
// the benchmark (also when it is killed): no language server outlives the
// run.
func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
}

// killGroup kills the process group of cmd.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
