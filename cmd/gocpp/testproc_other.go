//go:build !unix

package main

import (
	"os/exec"
	"syscall"
)

func setTestGroup(cmd *exec.Cmd) {}

// signalTest sends sig to the test binary; there are no process groups here, and a signal other
// than a kill is not supported by the system, so the process is killed.
func signalTest(cmd *exec.Cmd, sig syscall.Signal, group bool) {
	if cmd.Process == nil {
		return
	}
	if cmd.Process.Signal(sig) != nil {
		cmd.Process.Kill()
	}
}

const hasTestGroup = false
