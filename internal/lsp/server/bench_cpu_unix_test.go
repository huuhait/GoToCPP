//go:build unix

package server

import "syscall"

// processCPU returns the CPU time (user and system) that this process has
// used, in nanoseconds: on a loaded machine the wall time of a request says
// little, the work it costs does (background garbage collection counts too).
func processCPU() int64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return ru.Utime.Nano() + ru.Stime.Nano()
}
