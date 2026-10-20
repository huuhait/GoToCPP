// phase: 08

// A method called through an interface variable that the method sets to nil: the call keeps the dynamic value alive (ADR-086).
package main

import "fmt"

type Cn interface{ Bump() int }
type T struct {
	n    int
	name string
}

var gi Cn

func (t *T) Bump() int {
	gi = nil
	junk := make([]*T, 0)
	for i := 0; i < 10; i++ {
		junk = append(junk, &T{n: -1, name: "junkjunkjunk"})
	}
	t.n++
	return t.n + len(t.name) + len(junk)
}

func main() {
	gi = &T{n: 1, name: "abc"}
	fmt.Println(gi.Bump())
}

// Output:
// 15
