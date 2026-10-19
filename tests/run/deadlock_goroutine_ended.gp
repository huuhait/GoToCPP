// phase: 08
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive\]:\nmain\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

// The goroutine that was going to send ends instead (a Goexit, here), and main is left waiting:
// the check runs when a goroutine ends too.
package main

import (
	"fmt"
	"runtime"
)

func main() {
	ch := make(chan int)
	go func() {
		defer fmt.Println("the goroutine ends")
		runtime.Goexit()
		ch <- 1
	}()
	<-ch
}

// Output:
// the goroutine ends
