// phase: 08

// Methods that unlink their receiver from the structure that held its last other reference (list nodes, tree rotations, slice elements), then keep using it: the call must keep the receiver alive (ADR-086).
package main

import "fmt"

type Node struct {
	val        int
	prev, next *Node
	pad        [8]int
	name       string
}

type List struct {
	head, tail *Node
	n          int
}

var lst List

func push(v int) {
	n := &Node{val: v, name: fmt.Sprint("node", v)}
	if lst.tail == nil {
		lst.head, lst.tail = n, n
	} else {
		n.prev = lst.tail
		lst.tail.next = n
		lst.tail = n
	}
	lst.n++
}

func (n *Node) Unlink() int {
	if n.prev != nil {
		n.prev.next = n.next
	} else {
		lst.head = n.next
	}
	if n.next != nil {
		n.next.prev = n.prev
	} else {
		lst.tail = n.prev
	}
	n.prev, n.next = nil, nil
	lst.n--
	// churn memory so that a freed object would be reused
	junk := make([]*Node, 0, 8)
	for i := 0; i < 8; i++ {
		junk = append(junk, &Node{val: -1, name: "junk"})
	}
	s := 0
	for i := range n.pad {
		n.pad[i] = n.val * i
		s += n.pad[i]
	}
	return s + n.val + len(n.name) + len(junk)
}

// the receiver is not borrowed-by-ownership but the method of a value held only by lst
func (l *List) popFront() int {
	return l.head.Unlink()
}

func (l *List) popBack() int {
	if l.tail == nil {
		return -1
	}
	return l.tail.Unlink()
}

type TN struct {
	k    int
	l, r *TN
	name string
}
type Tree struct{ root *TN }

var tree Tree

func (n *TN) rotateLeftIn(t *Tree) int {
	nr := n.r
	n.r = nr.l
	nr.l = n
	t.root = nr
	n.r = nil
	nr.l = nil // now n has only the caller's reference
	junk := &TN{k: 99, name: "junk"}
	return n.k*100 + nr.k + len(n.name) + junk.k
}

func (n *TN) rotateRightIn(t *Tree) int {
	nl := n.l
	n.l = nl.r
	nl.r = n
	t.root = nl
	n.l = nil
	nl.r = nil
	return n.k*100 + nl.k + len(n.name)
}

func (t *Tree) rotate() int {
	return t.root.rotateLeftIn(t)
}

var ptrs []*Node

func (n *Node) swapOut(i, j int) int {
	ptrs[i], ptrs[j] = nil, nil
	ptrs = ptrs[:0]
	ptrs = append(ptrs, &Node{val: 7})
	return n.val + len(n.name)
}

func main() {
	for i := 1; i <= 6; i++ {
		push(i)
	}
	fmt.Println(lst.popFront(), lst.popBack(), lst.n)
	fmt.Println(lst.head.next.Unlink(), lst.n) // middle
	fmt.Println(lst.head.Unlink(), lst.head.Unlink(), lst.n)
	for lst.head != nil {
		fmt.Println(lst.head.Unlink())
	}
	fmt.Println(lst.popBack())

	tree.root = &TN{k: 1, name: "one", r: &TN{k: 2, name: "two", l: &TN{k: 3, name: "three"}}}
	fmt.Println(tree.rotate())
	fmt.Println(tree.root.k, tree.root.l == nil, tree.root.r == nil)
	tree.root = &TN{k: 5, name: "five", l: &TN{k: 4, name: "four"}}
	fmt.Println(tree.root.rotateRightIn(&tree))
	fmt.Println(tree.root.k)

	ptrs = []*Node{{val: 1, name: "a"}, {val: 2, name: "b"}, {val: 3, name: "c"}}
	fmt.Println(ptrs[1].swapOut(0, 2))
	fmt.Println(len(ptrs))
}

// Output:
// 42 187 4
// 100 3
// 71 129 1
// 158
// -1
// 204
// 2 true true
// 508
// 4
// 3
// 1
