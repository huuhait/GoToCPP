// phase: 09
// Stderr: ^close: <nil> fast: true leaked: 0 fd1-still-the-pipe: false\n$

//go:build linux

// File.Close of the original os.Stdout while a Println is stuck writing to descriptor 1 (a non-blocking pipe
// that nobody reads yet): Close returns at once, and what the Println has left to write does not land in the
// files opened after the close (which get descriptor 1 in Go; here the descriptor is closed by the Println
// that was in flight, when it has finished).
package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"time"
)

func main() {
	var p [2]int
	if err := syscall.Pipe(p[:]); err != nil {
		panic(err)
	}
	syscall.SetNonblock(p[1], true)
	var pipeStat syscall.Stat_t
	syscall.Fstat(p[0], &pipeStat)
	syscall.Dup2(p[1], 1)
	syscall.Close(p[1])
	big := strings.Repeat("B", 1<<20)
	done := make(chan bool)
	go func() {
		fmt.Println(big)
		close(done)
	}()
	time.Sleep(200 * time.Millisecond)
	t := time.Now()
	err := os.Stdout.Close()
	fast := time.Since(t) < 400*time.Millisecond
	var files []*os.File
	for i := 0; i < 5; i++ {
		f, err := os.CreateTemp("", "inflight")
		if err != nil {
			panic(err)
		}
		files = append(files, f)
	}
	// Now let the Println go on.
	go func() {
		r := os.NewFile(uintptr(p[0]), "pipe")
		io.Copy(io.Discard, r)
	}()
	<-done
	leaked := 0
	for _, f := range files {
		b, _ := os.ReadFile(f.Name())
		leaked += len(b)
		os.Remove(f.Name())
	}
	// Once the Println has ended, descriptor 1 is not the pipe any more (closed, or a file that took its number).
	var st syscall.Stat_t
	still := syscall.Fstat(1, &st) == nil && st.Ino == pipeStat.Ino
	fmt.Fprintln(os.Stderr, "close:", err, "fast:", fast, "leaked:", leaked, "fd1-still-the-pipe:", still)
}

// Output:
