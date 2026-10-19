//go:build unix

package main

import (
	"os/exec"
	"syscall"
)

// setTestGroup puts the test binary in a process group of its own, so that the backup alarm of
// gocpp test can end the processes that the tests started too (killTestGroup).
func setTestGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// signalTest sends sig to the test binary, and to its whole process group when group is set.
func signalTest(cmd *exec.Cmd, sig syscall.Signal, group bool) {
	if cmd.Process == nil {
		return
	}
	if group && syscall.Kill(-cmd.Process.Pid, sig) == nil {
		return
	}
	cmd.Process.Signal(sig)
}

// hasTestGroup reports whether setTestGroup does anything: the terminal's signals then do not
// reach the test binary and gocpp passes them on.
const hasTestGroup = true
