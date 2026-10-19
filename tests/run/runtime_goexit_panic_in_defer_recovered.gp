// phase: 08

// A deferred call that panics during Goexit, and a deferred call below it that recovers that panic:
// Goexit goes on afterwards, and the goroutine still ends there.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer fmt.Println("the outermost deferred call runs: Goexit goes on")
		defer func() {
			fmt.Println("recovered:", recover())
		}()
		defer func() {
			fmt.Println("this deferred call panics")
			panic("panic during Goexit")
		}()
		runtime.Goexit()
		fmt.Println("not reached")
	}()
	<-done
	fmt.Println("main continues")
}

// Output:
// this deferred call panics
// recovered: panic during Goexit
// the outermost deferred call runs: Goexit goes on
// main continues
