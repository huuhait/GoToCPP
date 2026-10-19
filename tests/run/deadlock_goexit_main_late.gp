// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: no goroutines \(main called runtime\.Goexit\) - deadlock!\n$

// Main calls Goexit and the goroutines that are left sleep and end: then no goroutine is left, and
// Go says so, without a trace.
package main

import (
	"fmt"
	"runtime"
	"time"
)

func main() {
	go func() {
		time.Sleep(30 * time.Millisecond)
		fmt.Println("the goroutine woke up and ended")
	}()
	runtime.Goexit()
}

// Output:
// the goroutine woke up and ended
