// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// A nil embedded pointer in the middle of a chain: reading and calling through the chain before
// it is fine, and the first access through the nil pointer panics with Go's message.
package main

import "fmt"

type Leaf struct{ V int }

func (l *Leaf) Ptr() bool { return l == nil }
func (l Leaf) Val() int   { return l.V }

type Mid struct {
	*Leaf
	M int
}

type Top struct {
	*Mid
	T int
}

func main() {
	t := Top{&Mid{nil, 1}, 2}
	fmt.Println(t.M, t.T, t.Mid.Leaf == nil, t.Ptr()) // Ptr is a pointer method: nil is fine
	t.Mid = nil
	fmt.Println(t.T, t.Mid == nil)
	fmt.Println(t.M) // t.Mid is nil
	fmt.Println("not reached")
}

// Output:
// 1 2 true true
// 2 true
