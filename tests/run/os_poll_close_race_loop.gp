// phase: 09

//go:build unix

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
)

// Close racing Read and Write in a loop: whatever the interleaving, a Read ends with the data or
// ErrClosed (never hangs, never another error), a Write with EPIPE or ErrClosed, and the descriptor
// is closed exactly once (no EBADF).
func main() {
	bad := 0
	for i := 0; i < 300; i++ {
		r, w, err := os.Pipe()
		if err != nil {
			panic(err)
		}
		var wg sync.WaitGroup
		var rerr, werr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			buf := make([]byte, 16)
			for {
				_, rerr = r.Read(buf)
				if rerr != nil {
					return
				}
			}
		}()
		go func() {
			defer wg.Done()
			for {
				if _, werr = w.Write([]byte("0123456789")); werr != nil {
					return
				}
			}
		}()
		for j := 0; j < i%7; j++ {
			runtime.Gosched()
		}
		switch i % 3 {
		case 0:
			r.Close()
		case 1:
			w.Close()
		default:
			go r.Close()
			w.Close()
		}
		wg.Wait()
		r.Close()
		w.Close()
		if rerr != nil && !errors.Is(rerr, os.ErrClosed) && rerr != io.EOF {
			bad++
			fmt.Println("read:", rerr)
		}
		if werr != nil && !errors.Is(werr, os.ErrClosed) && werr.Error() != "write |1: broken pipe" {
			bad++
			fmt.Println("write:", werr)
		}
	}
	fmt.Println("bad:", bad)

	// Goroutines blocked in Reads do not keep the program from ending.
	for i := 0; i < 5; i++ {
		r, _, _ := os.Pipe()
		go r.Read(make([]byte, 1))
	}
	runtime.Gosched()
	fmt.Println("exit")
}

// Output:
// bad: 0
// exit
