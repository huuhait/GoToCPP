// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// A nil pointer to an array: len, cap and a range without a value don't dereference it, an
// index does.
//go:build linux

package main

import "fmt"

func main() {
	var p *[3]int
	fmt.Println(len(p), cap(p))
	for i := range p {
		fmt.Print(i)
	}
	fmt.Println()
	p[0] = 1
}

// Output:
// 3 3
// 012
