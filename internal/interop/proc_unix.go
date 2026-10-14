//go:build unix

package interop

import (
	"os/exec"
	"syscall"
)

// inGroup makes cmd start a process group of its own, and makes canceling
// it (the time limit, a canceled probe, too much output) kill the whole
// group: g++ runs cc1plus as a child process, which would otherwise go on
// after g++ is killed. The language server cancels probes that are no
// longer needed, so their compilers must stop.
func inGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}

// killGroup kills what is left of the process group of cmd after cmd
// exited, so that nothing writes into the probe's folder while it is
// removed.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
