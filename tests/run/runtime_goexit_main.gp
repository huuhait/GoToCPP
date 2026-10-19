// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: no goroutines \(main called runtime\.Goexit\) - deadlock!\n

// The main goroutine calls Goexit: its deferred calls run, the other goroutines go on until they
// are done, and then the program ends with Go's fatal error and exit code 2 (nothing is left that
// could end it with code 0).
package main

import (
	"fmt"
	"runtime"
)

func main() {
	release := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		<-release
		fmt.Println("the other goroutine runs after main's deferred call")
		close(finished)
	}()
	defer func() {
		fmt.Println("main's deferred call")
		close(release)
	}()
	runtime.Goexit()
	fmt.Println("not reached")
}

// Output:
// main's deferred call
// the other goroutine runs after main's deferred call
