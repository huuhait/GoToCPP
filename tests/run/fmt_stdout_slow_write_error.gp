// phase: 09
// Stderr: (?m)^1: 0 write /dev/stdout: no space left on device$
// Stderr: (?m)^2: 0 write /dev/stdout: no space left on device$

//go:build linux

package main

import (
	"fmt"
	"os"
	"syscall"
)

func main() {
	saved := os.Stdout
	os.Stdout = saved // fmt now takes its slow path (the variable was assigned)
	full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		panic(err)
	}
	syscall.Dup2(int(full.Fd()), 1)
	n, err := fmt.Println("hello")
	fmt.Fprintln(os.Stderr, "1:", n, err)
	n, err = fmt.Printf("%d\n", 5)
	fmt.Fprintln(os.Stderr, "2:", n, err)
}

// Output:
