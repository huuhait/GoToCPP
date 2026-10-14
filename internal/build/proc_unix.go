//go:build unix

package build

import (
	"os/exec"
	"syscall"
)

// InGroup makes cmd start a process group of its own, and makes canceling
// it (a time limit, a canceled build or probe, too much output) kill the
// whole group: g++ runs cc1plus as a child process, which would otherwise
// go on after g++ is killed. The language server cancels namespace probes
// that are no longer needed, and gocpp build cancels its compiles when it
// is interrupted, so their compilers must stop.
//
// A process in a group of its own doesn't get the signals of the
// terminal (Ctrl-C) or of a kill of gocpp's group: whoever starts it
// must cancel it then.
func InGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}

// KillGroup kills what is left of the process group of cmd (started with
// InGroup) after cmd exited, so that nothing writes into a folder while
// it is removed.
func KillGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
