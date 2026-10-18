// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// A value method promoted through a nil embedded pointer panics at the call.
package main

import "fmt"

type Leaf struct{ V int }

func (l Leaf) Val() int { return l.V }

type Top struct {
	*Leaf
	n int
}

func main() {
	t := Top{nil, 1}
	fmt.Println("before")
	fmt.Println(t.Val())
}

// Output:
// before
