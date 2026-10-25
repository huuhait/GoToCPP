// phase: 08
// ExitCode: 2
// Stderr: ^panic: x\n\tpanic: panic called with nil argument\n\ngoroutine \d+ \[running\]:\n

// A deferred call recovers during Goexit (recover() returns nil there: Goexit is not a panic) and
// panics with that nil: Go's PanicNilError, chained to the aborted panic.
package main

import "runtime"

func main() {
	done := make(chan int)
	go func() {
		defer func() {
			r := recover()
			panic(r)
		}()
		defer runtime.Goexit()
		panic("x")
	}()
	<-done
}

// Output:
