// phase: 08

// Goexit called by a deferred call while a panic is running: the goroutine ends, the program does
// not crash, and the rest of the deferred calls run.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { fmt.Println("last deferred call runs, recover:", recover()) }()
		defer func() {
			fmt.Println("calls Goexit during the panic")
			runtime.Goexit()
		}()
		panic("boom")
	}()
	<-done
	fmt.Println("main survives")
}

// Output:
// calls Goexit during the panic
// last deferred call runs, recover: <nil>
// main survives
