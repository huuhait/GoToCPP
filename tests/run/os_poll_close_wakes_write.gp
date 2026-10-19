// phase: 09

//go:build unix

package main

import (
	"fmt"
	"os"
	"time"
)

// Close wakes a Write that is blocked on a full pipe: the Write returns the bytes the pipe took
// (a partial write) with "file already closed".
func main() {
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}
	type result struct {
		n   int
		err error
	}
	got := make(chan result)
	big := make([]byte, 4<<20) // far more than a pipe holds
	go func() {
		n, err := w.Write(big)
		got <- result{n, err}
	}()
	time.Sleep(200 * time.Millisecond) // the pipe is full, the Write waits
	fmt.Println("close:", w.Close())
	res := <-got
	fmt.Println(res.n > 0 && res.n < len(big), res.err)
	fmt.Println("close again:", w.Close())

	// The reader sees what was written, then EOF (the write end is closed).
	total := 0
	buf := make([]byte, 32<<10)
	for {
		n, err := r.Read(buf)
		total += n
		if err != nil {
			fmt.Println(err)
			break
		}
	}
	fmt.Println(total == res.n)
	r.Close()

	// A Write after the reader closed: EPIPE (not SIGPIPE, the file is not stdout).
	r, w, _ = os.Pipe()
	r.Close()
	_, err = w.Write([]byte("x"))
	fmt.Println(err)
	w.Close()
}

// Output:
// close: <nil>
// true write |1: file already closed
// close again: close |1: file already closed
// EOF
// true
// write |1: broken pipe
