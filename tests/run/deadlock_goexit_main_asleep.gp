// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine \d+ \[chan receive\]:\n(?s:.*)created by main\.main in goroutine 1\n\t.*:\d+ \+0x[0-9a-f]+\n$

// Main calls Goexit while another goroutine waits for ever: the others are all asleep, the report
// lists them (main is gone and is not in it), and it is the "all goroutines are asleep" error, not
// the "no goroutines" one.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	go func() { <-make(chan int) }()
	defer fmt.Println("main's deferred call")
	runtime.Goexit()
}

// Output:
// main's deferred call
