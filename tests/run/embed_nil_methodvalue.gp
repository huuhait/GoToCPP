// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// A method value of a value method through a nil embedded pointer panics where the value is
// made (the receiver is copied then), not where it is called.
package main

import "fmt"

type Leaf struct{ V int }

func (l Leaf) Val() int { return l.V }

type Top struct{ *Leaf }

func main() {
	var t Top
	fmt.Println("making")
	f := t.Val
	fmt.Println("made")
	fmt.Println(f())
}

// Output:
// making
