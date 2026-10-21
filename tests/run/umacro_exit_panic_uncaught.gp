// phase: 05
// ExitCode: 2
// Stderr: (?m)^panic: boom

// The exit part of a start-style macro runs when nothing recovers the panic (exit criterion of
// Phase 10): the function's own defer runs first, then the macro's exit part, and only then does the
// program die with exit status 2 and the panic message. The twin is
// testdata/macro_twin/umacro_exit_panic_uncaught.go.
package main

import "fmt"

// Trace prints when the function starts and when it ends.
macro Trace() func() {
	fmt.Println("enter", #func())
	return func() { fmt.Println("exit ", #func()) }
}

#Trace()
func explode() {
	defer fmt.Println("own defer of explode")
	panic("boom")
}

func main() {
	fmt.Println("before")
	explode()
	fmt.Println("not reached")
}

// Output:
// before
// enter main.explode
// own defer of explode
// exit  main.explode
