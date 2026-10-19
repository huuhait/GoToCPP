// phase: 09

//go:build unix

// A goroutine that waits in a Read on a pipe is not asleep for the deadlock detector: with main
// parked on a channel and nothing else running, the Read's deadline (the wait ends by itself) wakes
// it and it answers main; no "all goroutines are asleep". The same for a Write on a full pipe.
package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	r, w, _ := os.Pipe()
	done := make(chan string)
	go func() {
		r.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
		_, err := r.Read(make([]byte, 1))
		done <- fmt.Sprint(err)
	}()
	fmt.Println(<-done)
	go func() {
		w.SetWriteDeadline(time.Now().Add(300 * time.Millisecond))
		n, err := w.Write(make([]byte, 1<<20))
		done <- fmt.Sprint(n > 0, err)
	}()
	fmt.Println(<-done)
	r.Close()
	w.Close()
}

// Output:
// read |0: i/o timeout
// true write |1: i/o timeout
