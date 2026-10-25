// phase: 08
// flags: -release
// ExitCode: 2
// Stderr: ^panic: x\n\tpanic: y \[recovered\]\n\tpanic: outer\n$

// A -release program: two panics in a chain, the deferred call that recovers the second runs
// Goexit, then the last deferred call panics: both aborted panics are printed, the second marked
// recovered, and nothing else.
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
