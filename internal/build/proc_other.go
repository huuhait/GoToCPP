//go:build !unix

package build

import "os/exec"

// Without Unix process groups, canceling a command kills only the
// compiler, not the processes that it started (on Windows a job object
// could kill the whole tree; the Windows port in Phase 14 can add it).
// The callers' WaitDelay still makes sure that they don't wait for them.

// InGroup does nothing here (see proc_unix.go).
func InGroup(*exec.Cmd) {}

// KillGroup does nothing here (see proc_unix.go).
func KillGroup(*exec.Cmd) {}
