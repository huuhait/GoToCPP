// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// A store into a field through a nil pointer panics after the value is evaluated, as in Go.
//go:build linux

package main

import "fmt"

type Pair struct{ a, b int }

func value() int {
	fmt.Println("value")
	return 3
}

func main() {
	var p *Pair
	fmt.Println(p == nil)
	p.b = value()
}

// Output:
// true
// value
