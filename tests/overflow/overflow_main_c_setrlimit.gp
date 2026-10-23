// phase: 04
//go:build linux
// ExitCode: 2
// Stderr: (?m)^fatal error: stack overflow$
// Stderr: (?m)^goroutine 1 \[running\]:$

// C code lowers the stack limit, which the runtime does not hear of: its bounds of the main stack
// still say 1 GiB (or the hard limit). The overflow handler takes a fault of the main thread just
// below its stack pointer as an overflow anyway (thread.cpp, main_overflow), so it is Go's report,
// not a bare SIGSEGV. Not a Go program.
package main

import (
	"<sys/resource.h>"
	"fmt"
)

var sink [4]int

func recurse(n int) int {
	sink[n%4] = n
	return recurse(n+1) + sink[(n+1)%4]
}

func main() {
	var r resource.rlimit
	resource.getrlimit(resource.RLIMIT_STACK, &r)
	r.rlim_cur = 8 << 20
	fmt.Println(resource.setrlimit(resource.RLIMIT_STACK, &r))
	fmt.Println(recurse(0))
}

// Output:
// 0
