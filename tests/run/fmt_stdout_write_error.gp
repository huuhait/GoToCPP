// phase: 09
// Stderr: (?m)^1: 0 write /dev/stdout: no space left on device$
// Stderr: (?m)^2: 0 write /dev/stdout: no space left on device$
// Stderr: (?m)^3: 0 write /dev/stdout: bad file descriptor$
// Stderr: (?m)^4: 0 write /dev/stdout: bad file descriptor$
// Stderr: (?m)^5: 0 write /dev/stdout: bad file descriptor$

//go:build linux

// A Print whose write to descriptor 1 fails returns 0 and the error, as Go's does (a full disk, a closed descriptor).
package main

import (
	"fmt"
	"os"
	"syscall"
)

func main() {
	full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		panic(err)
	}
	syscall.Dup2(int(full.Fd()), 1)
	n, err := fmt.Println("hello")
	fmt.Fprintln(os.Stderr, "1:", n, err)
	n, err = fmt.Printf("%d\n", 5)
	fmt.Fprintln(os.Stderr, "2:", n, err)
	syscall.Close(1)
	n, err = fmt.Println("after close of fd 1")
	fmt.Fprintln(os.Stderr, "3:", n, err)
	n, err = fmt.Print("x")
	fmt.Fprintln(os.Stderr, "4:", n, err)
	n, err = fmt.Print()
	fmt.Fprintln(os.Stderr, "5:", n, err)
}

// Output:
