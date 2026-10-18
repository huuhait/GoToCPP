// phase: 05

// A pointer method promoted through a nil embedded pointer is called with a nil receiver, as
// in Go; only a dereference inside it panics.
package main

import "fmt"

type Leaf struct{ V int }

func (l *Leaf) IsNil() bool { return l == nil }
func (l *Leaf) Safe() int {
	if l == nil {
		return -1
	}
	return l.V
}

type Mid struct{ *Leaf }
type Top struct{ Mid }

func main() {
	var t Top
	fmt.Println(t.IsNil(), t.Safe(), t.Mid.IsNil(), t.Leaf == nil)
	f := t.Safe
	g := (*Top).IsNil
	fmt.Println(f(), g(&t))
	t.Leaf = &Leaf{5}
	fmt.Println(t.IsNil(), t.Safe(), f(), g(&t))
	defer fmt.Println(t.Safe())
	t.Leaf = nil
}

// Output:
// true -1 true true
// -1 true
// false 5 -1 false
// 5
