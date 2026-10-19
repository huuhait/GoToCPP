// phase: 09

//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func main() {
	r, w, _ := os.Pipe()
	r.Close()
	n, err := w.Write([]byte("nobody reads"))
	fmt.Println(n, err)
	pe := err.(*os.PathError)
	fmt.Println(pe.Op, pe.Path, pe.Err == syscall.EPIPE, errors.Is(err, syscall.EPIPE))
	n, err = w.WriteString("again")
	fmt.Println(n, err)
	w.Close()
}

// Output:
// 0 write |1: The pipe is being closed.
// write |1 false false
// 0 write |1: The pipe is being closed.
