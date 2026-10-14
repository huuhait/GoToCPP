// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// Dereferencing a nil pointer panics with Go's message and the signal line (exit code 2),
// after the calls of its statement ran.
//go:build linux

package main

import "fmt"

func called() int {
	fmt.Println("called")
	return 1
}

func main() {
	x := 1
	p := &x
	fmt.Println(*p, p != nil)
	p = nil
	fmt.Println(p == nil)
	fmt.Println(*p + called())
}

// Output:
// 1 true
// true
// called
