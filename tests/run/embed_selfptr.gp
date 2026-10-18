// phase: 05

// A struct that embeds a pointer to itself: the field is named like the struct, which C++ would
// read as a constructor, and the promoted fields and methods are reached through the chain.
package main

import "fmt"

type Node struct {
	*Node
	v int
}

func (n *Node) Val() int { return n.v }

func (n Node) Depth() int {
	if n.Node == nil {
		return 1
	}
	return 1 + n.Node.Depth()
}

func main() {
	n := &Node{&Node{&Node{nil, 3}, 2}, 1}
	fmt.Println(n.Val(), n.Node.Val(), n.Node.Node.Val(), n.Depth())
	fmt.Println(n.v, n.Node.v, n.Node.Node.v, n.Node.Node.Node == nil)
	n.Node.Node.Node = &Node{nil, 4}
	fmt.Println(n.Depth(), n.Node.Node.Node.v)
}

// Output:
// 1 2 3 3
// 1 2 3 true
// 4 4
