// phase: 08
// flags: -release
// ExitCode: 2
// Stderr: ^panic: x\n\tpanic: outer\n$

// A -release program: runtime.Goexit runs during a panic, then a deferred call panics: the report
// is the message lines only, with the aborted panic first, as Go prints them.
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
