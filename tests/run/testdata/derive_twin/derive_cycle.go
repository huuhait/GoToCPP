package main

import "fmt"

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

func (n Node) Equal(o Node) bool {
	if n.V != o.V {
		return false
	}
	for _, p := range [][2]*Node{{n.Next, o.Next}, {n.Left, o.Left}, {n.Right, o.Right}} {
		if p[0] == p[1] {
			continue
		}
		if p[0] == nil || p[1] == nil || !p[0].Equal(*p[1]) {
			return false
		}
	}
	return true
}

func (n Node) Clone() Node {
	out := n
	for _, p := range []**Node{&out.Next, &out.Left, &out.Right} {
		if *p != nil {
			c := (*p).Clone()
			*p = &c
		}
	}
	return out
}
