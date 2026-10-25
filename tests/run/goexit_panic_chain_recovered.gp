// phase: 08
// ExitCode: 2
// Stderr: ^panic: x \[recovered\]\n\tpanic: outer\n\ngoroutine \d+ \[running\]:\n

// The deferred call that runs runtime.Goexit recovers the panic first: the aborted panic is
// printed with " [recovered]" before the panic of the next deferred call.
package main

import "runtime"

func main() {
	done := make(chan int)
	go func() {
		defer func() { panic("outer") }()
		defer func() {
			recover()
			runtime.Goexit()
		}()
		panic("x")
	}()
	<-done
}

// Output:
