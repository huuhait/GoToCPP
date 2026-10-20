// phase: 09
// Stderr: ^leaked bytes: 0\n$

//go:build linux

// File.Close of the original os.Stdout while goroutines print: a Print that began before the close must
// not write to descriptor 1 after the close, where the next file opened (os.CreateTemp) may be.
package main

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
)

func main() {
	// Descriptor 1 is /dev/null while the printers run (what they print before the close goes nowhere).
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		panic(err)
	}
	syscall.Dup2(int(null.Fd()), 1)
	null.Close()
	var wg sync.WaitGroup
	var started atomic.Int32
	stop := make(chan struct{})
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			started.Add(1)
			for {
				select {
				case <-stop:
					return
				default:
				}
				fmt.Println("LEAK")
			}
		}()
	}
	for started.Load() < 8 {
	}
	os.Stdout.Close()
	var files []*os.File
	for i := 0; i < 30; i++ {
		f, err := os.CreateTemp("", "toctou")
		if err != nil {
			panic(err)
		}
		files = append(files, f)
	}
	close(stop)
	wg.Wait()
	leaked := 0
	for _, f := range files {
		b, _ := os.ReadFile(f.Name())
		leaked += len(b)
		os.Remove(f.Name())
	}
	fmt.Fprintf(os.Stderr, "leaked bytes: %d\n", leaked)
}

// Output:
