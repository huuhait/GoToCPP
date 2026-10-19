package main

import (
	"os/exec"
	"syscall"
)

// setPdeathsig makes the kernel kill cmd when the runner dies, however it dies (the process
// group is killed by the runner itself when it ends in order).
func setPdeathsig(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Pdeathsig = syscall.SIGKILL
}
