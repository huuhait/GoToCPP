package cputime

import (
	"syscall"
	"time"
	"unsafe"
)

// clockThreadCPUTime is CLOCK_THREAD_CPUTIME_ID of <time.h>.
const clockThreadCPUTime = 3

// threadTime returns the processor time of the calling thread.
func threadTime() (time.Duration, bool) {
	var ts syscall.Timespec
	if _, _, errno := syscall.Syscall(syscall.SYS_CLOCK_GETTIME, clockThreadCPUTime, uintptr(unsafe.Pointer(&ts)), 0); errno != 0 {
		return 0, false
	}
	return time.Duration(ts.Nano()), true
}
