// phase: 03
// ExitCode: 2
// Stderr: ^start\npanic: runtime error: index out of range \[5\] with length 3\n\ngoroutine 1 \[running\]:\nmain\.at\(\.\.\.\)\n\t/.+/panic_trace\.g[op]:14( \+0x[0-9a-f]+)?\nmain\.last\((\.\.\.)?\)\n\t/.+/panic_trace\.g[op]:18( \+0x[0-9a-f]+)?\nmain\.main\(\)\n\t/.+/panic_trace\.g[op]:23 \+0x[0-9a-f]+\n$

// An unrecovered panic prints Go's lines, then the stack trace: each function by its Go name,
// "(...)" when it has parameters, and the .gp file and line of the call with the offset in the
// function. gocpp's own runtime frames are left out, as Go leaves out its runtime's. (Go
// inlines at and last: it prints "(...)" and no offset for an inlined frame.)
package main

import "fmt"

func at(s string, i int) byte {
	return s[i]
}

func last() byte {
	return at("abc", 5)
}

func main() {
	println("start")
	fmt.Println(last())
}

// Output:
