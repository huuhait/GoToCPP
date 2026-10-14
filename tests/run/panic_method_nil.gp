// phase: 03
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\nmain\.main\(\)\n

// A value method called through a nil pointer panics at the call (step 3.10), in main, before
// the method runs: Go dereferences the pointer to copy the receiver.
package main

import "fmt"

type Node struct {
	v    int
	next *Node
}

func (n Node) Value() int {
	fmt.Println("not reached")
	return n.v
}

func main() {
	n := &Node{v: 1, next: &Node{v: 2}}
	for ; ; n = n.next {
		fmt.Println(n.Value())
	}
}

// Output:
// not reached
// 1
// not reached
// 2
