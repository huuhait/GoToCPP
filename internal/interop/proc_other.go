//go:build !unix

package interop

import "os/exec"

// Without Unix process groups, canceling a probe kills only the compiler,
// not the processes that it started (on Windows a job object could kill
// the whole tree; the Windows port in Phase 14 can add it). The probe's
// WaitDelay still makes sure that the probe doesn't wait for them.

func inGroup(*exec.Cmd)   {}
func killGroup(*exec.Cmd) {}
