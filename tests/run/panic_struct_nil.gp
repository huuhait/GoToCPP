// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// A field through a nil pointer panics with Go's message (Go's signal line shows the field's
// offset, gocpp's 0x0). The pointer is checked where the field is read, after the calls before
// it.
//go:build linux

package main

import "fmt"

type Node struct {
	val  int
	next *Node
}

func two() int {
	fmt.Println("two runs first")
	return 2
}

func main() {
	n := &Node{val: 1}
	fmt.Println(n.val, n.next == nil)
	fmt.Println(n.next.val + two())
}

// Output:
// 1 true
// two runs first
