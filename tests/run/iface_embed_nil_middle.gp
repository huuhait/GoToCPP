// phase: 05
// ExitCode: 2
// Stderr: ^panic: runtime error: invalid memory address or nil pointer dereference\n\[signal SIGSEGV: segmentation violation code=0x1 addr=0x[0-9a-f]+ pc=0x[0-9a-f]+\]\n\ngoroutine 1 \[running\]:\n

// A nil pointer in the middle of the path of a promoted method called through an interface: a pointer
// method called through a nil embedded pointer gets the nil receiver (and works), a value method
// derefs it and panics.
package main

import "fmt"

type Valuer interface{ Val() int }
type Checker interface{ IsNil() bool }

type Leaf struct{ v int }

func (l Leaf) Val() int      { return l.v }
func (l *Leaf) IsNil() bool { return l == nil }

type Mid struct{ *Leaf }
type Top struct{ Mid }

func main() {
	var t Top
	var c Checker = t
	var v Valuer = t
	fmt.Println(c.IsNil())
	t.Leaf = &Leaf{4}
	fmt.Println(c.IsNil(), Checker(t).IsNil(), Valuer(t).Val())
	fmt.Println(v.Val())
}

// Output:
// true
// true false 4
