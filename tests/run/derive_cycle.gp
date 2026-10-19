// phase: 10
// lsan: cycle

// Types that refer to themselves, and what the generators do not do about references (step
// 10.10): Equal and Clone follow pointers all the way, as the code that a Go programmer writes by
// hand does. A list or a tree is fine, and so is a value that is shared (two fields that point to
// one node): Equal compares what the pointers point at, and Clone makes a copy for each pointer,
// so a shared node becomes two. A value that points back to itself is not supported: Clone of it
// and Equal of two different ones overflow the stack, like the hand-written code (ADR-080; the
// test cannot show it, because a sanitizer run reports a stack overflow in its own words).
// The twin (testdata/derive_twin/derive_cycle.go) has the methods written by hand.

package main

import "fmt"

#Derive(Clone, Equal)
type Node struct {
	V           int
	Next        *Node
	Left, Right *Node
}

func main() {
	// a cycle is equal to itself at once: the same pointer
	a := &Node{V: 1}
	a.Next = a
	fmt.Println(a.Equal(*a), a.Equal(Node{V: 1, Next: a}))

	// a list and a tree
	list := Node{V: 1, Next: &Node{V: 2, Next: &Node{V: 3}}}
	c := list.Clone()
	c.Next.Next.V = 30
	fmt.Println(list.Next.Next.V, c.Next.Next.V, list.Equal(c), list.Equal(list.Clone()))

	// a shared node: one node reached twice
	shared := &Node{V: 7}
	dag := Node{V: 0, Left: shared, Right: shared}
	d := dag.Clone()
	fmt.Println(dag.Left == dag.Right, d.Left == d.Right, d.Left.V, d.Right.V, dag.Equal(d))
	d.Left.V = 8
	fmt.Println(d.Left.V, d.Right.V, shared.V, dag.Equal(d))
}

// Output:
// true true
// 3 30 false true
// true false 7 7 true
// 8 7 7 false
