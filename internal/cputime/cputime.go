// Package cputime measures the processor time of a piece of work, for tests that check that
// something is not quadratic or slow: unlike the wall clock, the processor time of a thread does
// not grow while the machine is loaded and the thread waits for a processor.
package cputime

import (
	"runtime"
	"time"
)

// Of runs f on an OS thread of its own (runtime.LockOSThread) and returns the processor time
// that thread used, user and system. Goroutines that f starts run on other threads and are not
// counted, nor are the garbage collector's background workers. Precise reports whether the time
// is the thread's processor time: where the system has no clock for it, Of returns the wall time
// of f and false.
func Of(f func()) (d time.Duration, precise bool) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	start, ok := threadTime()
	wall := time.Now()
	f()
	if !ok {
		return time.Since(wall), false
	}
	end, ok := threadTime()
	if !ok {
		return time.Since(wall), false
	}
	return end - start, true
}
