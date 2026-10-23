// phase: 09
//go:build linux
// ExitCode: 2
// Stderr: (?m)^fatal error: stack overflow$
// Stderr: (?m)^goroutine 1 \[running\]:$

// The program lowers its own stack limit (syscall.Setrlimit) and then recurses without end: the
// runtime takes the new limit as the end of the main goroutine's stack (sys_setrlimit computes the
// bounds again), so the overflow is Go's report, not a bare SIGSEGV. (Go's stacks don't follow the
// limit; it reports the overflow at 1 GB.)
package main

import (
	"fmt"
	"syscall"
)

var sink [4]int

func recurse(n int) int {
	sink[n%4] = n
	return recurse(n+1) + sink[(n+1)%4]
}

func main() {
	var r syscall.Rlimit
	fmt.Println(syscall.Getrlimit(syscall.RLIMIT_STACK, &r))
	r.Cur = 8 << 20
	fmt.Println(syscall.Setrlimit(syscall.RLIMIT_STACK, &r))
	fmt.Println(recurse(0))
}

// Output:
// <nil>
// <nil>
