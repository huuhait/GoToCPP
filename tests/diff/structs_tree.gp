// phase: 03

// A binary search tree with deletion, through pointers to pointers to nodes, and a tree whose
// children are a slice of nodes, which is flattened by assignments whose value is reached from
// their target (t = t.kids[0]): the value is copied before the target's old fields are freed.
package main

import "fmt"

type node struct {
	key         int
	left, right *node
}

func insert(pp **node, key int) {
	for *pp != nil {
		if key < (*pp).key {
			pp = &(*pp).left
		} else {
			pp = &(*pp).right
		}
	}
	*pp = &node{key: key}
}

func remove(pp **node, key int) bool {
	for *pp != nil && (*pp).key != key {
		if key < (*pp).key {
			pp = &(*pp).left
		} else {
			pp = &(*pp).right
		}
	}
	n := *pp
	switch {
	case n == nil:
		return false
	case n.left == nil:
		*pp = n.right
	case n.right == nil:
		*pp = n.left
	default:
		sp := &n.right
		for (*sp).left != nil {
			sp = &(*sp).left
		}
		s := *sp
		*sp = s.right
		n.key = s.key
	}
	return true
}

func show(n *node) {
	if n == nil {
		fmt.Print(".")
		return
	}
	fmt.Print("(")
	show(n.left)
	fmt.Print(" ", n.key, " ")
	show(n.right)
	fmt.Print(")")
}

type tree struct {
	label string
	kids  []tree
}

func main() {
	var root *node
	for _, k := range []int{50, 30, 70, 20, 40, 60, 80, 35, 45, 65} {
		insert(&root, k)
	}
	show(root)
	fmt.Println()
	for _, k := range []int{30, 50, 99, 20, 70} {
		fmt.Print(k, " ", remove(&root, k), " ")
		show(root)
		fmt.Println()
	}

	t := tree{label: "a", kids: []tree{{label: "b", kids: []tree{{label: "c", kids: []tree{{label: "d"}}}}}}}
	for len(t.kids) > 0 {
		fmt.Print(t.label, " ")
		t = t.kids[0]
	}
	fmt.Println(t.label)
	ts := []tree{{label: "x", kids: []tree{{label: "y"}}}}
	ts[0] = ts[0].kids[0]
	fmt.Println(ts[0].label, len(ts[0].kids))
}
