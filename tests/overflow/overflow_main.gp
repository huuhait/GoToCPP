// phase: 08
// ExitCode: 2
// Stderr: (?m)^fatal error: stack overflow$
// Stderr: (?m)^goroutine \d+ .*\[running\]:$

// Endless recursion on the main goroutine ends with Go's "fatal error: stack overflow" and exit code 2
// (not a bare SIGSEGV). It cannot be recovered. (gocpp's main stack grows up to 1 GiB, Go's limit
// is 1 GB; TestStackOverflow runs it with a hard limit of 64 MiB, so it ends there.)
package main

import "fmt"

var sink [4]int

func recurse(n int) int {
	sink[n%4] = n
	return recurse(n+1) + sink[(n+1)%4]
}

func main() {
	defer fmt.Println("deferred calls do not run after a fatal error")
	defer func() { recover() }()
	fmt.Println(recurse(0))
}

// Output:
