//go:build !unix

package tests

import "os/exec"

// Without Unix process groups, canceling a command kills only the command,
// not the processes that it started, and a signal ends the test binary
// without cleaning up. (On Windows a job object can kill a whole process
// tree; the Windows port in Phase 14 can add it.)

func inGroup(*exec.Cmd)   {}
func track(int, bool)     {}
func killGroup(int) error { return nil }
func stopRunning()        {}
func isStopping() bool    { return false }
func killOnSignal(string) {}
