// phase: 09

//go:build linux

package main

import (
	"fmt"
	"os"
	"time"
)

func fds() int {
	ents, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		panic(err)
	}
	return len(ents)
}

// Close of a file with a Read in flight waits until the descriptor is closed (Go: the Read has
// returned by then): the process has the same descriptors before the pipe and after Close.
func main() {
	// The first pollable file sets Go's poller up (an epoll descriptor that stays open).
	r0, w0, _ := os.Pipe()
	r0.Close()
	w0.Close()
	base := fds()
	bad := 0
	for i := 0; i < 20; i++ {
		r, w, _ := os.Pipe()
		done := make(chan struct{})
		go func() {
			r.Read(make([]byte, 1))
			close(done)
		}()
		time.Sleep(time.Duration(i%3) * 5 * time.Millisecond)
		r.Close()
		if fds() != base+1 { // the write end is still open
			bad++
		}
		w.Close()
		<-done
		if fds() != base {
			bad++
		}
	}
	fmt.Println("bad:", bad)
}

// Output:
// bad: 0
