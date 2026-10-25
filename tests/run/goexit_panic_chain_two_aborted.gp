// phase: 08
// ExitCode: 2
// Stderr: ^panic: x\n\tpanic: y \[recovered\]\n\tpanic: outer\n\ngoroutine \d+ \[running\]:\n

// Two panics in a chain (x, then y in a deferred call), the deferred call that recovers y runs
// Goexit: both are aborted, and they are printed in order, y marked recovered, before the panic
// of the last deferred call.
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
		defer func() { panic("y") }()
		panic("x")
	}()
	<-done
}

// Output:
