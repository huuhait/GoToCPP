// phase: 09
// ExitCode: 2
// Stderr: ^fatal error: all goroutines are asleep - deadlock!\n\ngoroutine 1 \[chan receive\]:\nmain\.main\(\)\n\t.*:\d+ \+0x[0-9a-f]+\n$

//go:build unix

// A goroutine that waits in a Read on a pipe is not asleep for the deadlock detector (Go: the
// poller may still wake it); once Close has ended the Read and the goroutine is gone, a main that
// waits forever is the deadlock, reported as usual.
package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	r, _, _ := os.Pipe()
	done := make(chan error)
	go func() {
		_, err := r.Read(make([]byte, 1))
		done <- err
	}()
	time.Sleep(100 * time.Millisecond)
	r.Close()
	fmt.Println(<-done)
	<-make(chan int)
}

// Output:
// read |0: file already closed
