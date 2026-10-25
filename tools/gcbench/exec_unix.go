//go:build unix

package main

import (
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

// setupChild makes the child (and its children) die with the runner: its own
// process group, killed as a whole on a timeout, and on Linux SIGKILL when the
// runner dies (Pdeathsig), so a killed runner leaves no busy benchmark loop.
func setupChild(cmd *exec.Cmd) {
	attr := &syscall.SysProcAttr{Setpgid: true}
	setPdeathsig(attr)
	cmd.SysProcAttr = attr
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second
}

// usage returns the CPU time (user + system) and wait4's peak resident set in
// KiB of a finished process.
func usage(ps *os.ProcessState) (cpu time.Duration, rssKB int64) {
	ru, ok := ps.SysUsage().(*syscall.Rusage)
	if !ok || ru == nil {
		return ps.UserTime() + ps.SystemTime(), 0
	}
	rss := int64(ru.Maxrss)
	if runtime.GOOS == "darwin" {
		rss /= 1024 // bytes there
	}
	return ps.UserTime() + ps.SystemTime(), rss
}
