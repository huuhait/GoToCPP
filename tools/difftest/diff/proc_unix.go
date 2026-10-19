//go:build unix

package diff

import (
	"os/exec"
	"syscall"
)

// inGroup makes cmd a process group of its own that dies with the program (Pdeathsig where
// there is one) and is killed as a whole when the context ends: no compiler or test program
// outlives the campaign.
func inGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	setPdeathsig(cmd.SysProcAttr)
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}

func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// terminate asks cmd to stop: SIGTERM to the process itself (not its group): gocpp stops its
// compilers, which run in process groups of their own, removes its work folder and dies; a
// SIGKILL of gocpp's group would leave the compilers running for half a minute.
func terminate(cmd *exec.Cmd) error { return syscall.Kill(cmd.Process.Pid, syscall.SIGTERM) }
