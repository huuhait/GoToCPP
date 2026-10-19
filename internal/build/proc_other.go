//go:build !unix && !windows

package build

import "os/exec"

// Without Unix process groups or Windows job objects (proc_windows.go), canceling a command kills
// only the compiler, not the processes that it started. The callers' WaitDelay still makes sure
// that they don't wait for them.

// InGroup does nothing here (see proc_unix.go).
func InGroup(*exec.Cmd) {}

// KillGroup does nothing here (see proc_unix.go).
func KillGroup(*exec.Cmd) {}

// joinGroup does nothing here (see proc_unix.go).
func joinGroup(*exec.Cmd) {}
