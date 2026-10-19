// phase: 09

//go:build linux

package main

import (
	"fmt"
	"io"
	"os"
	"syscall"
)

func main() {
	var p [2]int
	if err := syscall.Pipe2(p[:], syscall.O_NONBLOCK|syscall.O_CLOEXEC); err != nil {
		panic(err)
	}
	r := os.NewFile(uintptr(p[0]), "r")
	w := os.NewFile(uintptr(p[1]), "w")
	const total = 3 << 20
	done := make(chan int64)
	go func() {
		n, err := io.Copy(io.Discard, r)
		if err != nil {
			panic(err)
		}
		done <- n
	}()
	chunk := make([]byte, 1<<20)
	for i := 0; i < total/len(chunk); i++ {
		n, err := w.Write(chunk)
		if err != nil || n != len(chunk) {
			panic(fmt.Sprint(n, err))
		}
	}
	w.Close()
	fmt.Println(<-done)
	r.Close()
}

// Output:
// 3145728
