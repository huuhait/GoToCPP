// phase: 08
// ExitCode: 2
// Stderr: ^panic: x\n\tpanic: outer\n\ngoroutine \d+ \[running\]:\n

// runtime.Goexit runs in a deferred call while a panic runs: the panic is aborted, not recovered,
// and Go keeps it in the goroutine's list of panics. A deferred call that panics afterwards links
// to it, and the report prints the aborted panic first ("panic: x" then "\tpanic: outer"); the
// Goexit itself is not printed.
package main

import "runtime"

func main() {
	done := make(chan int)
	go func() {
		defer func() { panic("outer") }()
		defer runtime.Goexit()
		panic("x")
	}()
	<-done
}

// Output:
