// phase: 08

// runtime.Goexit ends the calling goroutine after it has run its deferred calls, last one
// first. Nothing after the call runs.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer fmt.Println("deferred 1 (runs last)")
		defer fmt.Println("deferred 2")
		defer func() { fmt.Println("deferred 3 (runs first)") }()
		runtime.Goexit()
		fmt.Println("not reached")
	}()
	<-done
	fmt.Println("main continues")
}

// Output:
// deferred 3 (runs first)
// deferred 2
// deferred 1 (runs last)
// main continues
