//go:build unix

package main

import (
	"os/exec"
	"syscall"
	"time"
)

// inGroup makes cmd start a process group of its own, die with this
// process where the system can do that (Linux), and stop as a whole when it
// is canceled: SIGTERM first (gocpp removes its work folder on it), SIGKILL
// two seconds later (cmd.WaitDelay).
func inGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	setPdeathsig(cmd.SysProcAttr)
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
	cmd.WaitDelay = 2 * time.Second
}

// killGroup kills what the command left running (a program that forked).
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// noCoreFiles turns core dumps off for this process and its children.
func noCoreFiles() {
	syscall.Setrlimit(syscall.RLIMIT_CORE, &syscall.Rlimit{})
}
