// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive, locked to thread\]:\n(?:.+\n\t.+\n)*(?:main\.init\.0|main\.init|\?)\(\)\n\t

// The package initializers run on the main goroutine locked to its thread: a deadlock in init
// says so (after init it is unlocked again: deadlock_chan_recv.gp's goroutine 1 does not).
package main

import "fmt"

var c = make(chan int)

func init() {
	fmt.Println("init waits")
	<-c
}

func main() {
	fmt.Println("not reached")
}

// Output:
// init waits
