// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: no goroutines \(main called runtime\.Goexit\) - deadlock!\n$

// An armed AfterFunc starts a goroutine, so Goexit of main does not end the program yet.
package main

import (
	"fmt"
	"runtime"
	"time"
)

func main() {
	time.AfterFunc(30*time.Millisecond, func() { fmt.Println("the timer function ran") })
	runtime.Goexit()
}

// Output:
// the timer function ran
