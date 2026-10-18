// phase: 04

// The operations of a linked list of C structs that gocpp builds, as Go code writes them: push, pop,
// reverse in place, remove from the middle, splice, move a node between lists, swap, copy a struct,
// a slice of nodes, an array member, nested members, and a cycle that is broken by hand. The nodes
// are C structs (umem.ucounted counts the live ones); the pointers in their members are raw, and
// each read gives a gocpp pointer that owns the node again. Under ASan, one that doesn't is a
// heap-use-after-free, and a node that is not freed is a leak.
package main

import (
	"fmt"

	"./umem.hpp"
)

type List struct{ head *umem.ucounted }

func (l *List) Push(id int) {
	n := &umem.ucounted{id: C.int(id)}
	n.next = l.head
	l.head = n
}

func (l *List) Pop() int {
	n := l.head
	l.head = n.next // the read gives a pointer that owns the second node
	return int(n.id)
}

func (l *List) Reverse() {
	var prev *umem.ucounted
	for cur := l.head; cur != nil; {
		next := cur.next
		cur.next = prev
		prev = cur
		cur = next
	}
	l.head = prev
}

func (l *List) Remove(id int) bool {
	var prev *umem.ucounted
	for cur := l.head; cur != nil; cur = cur.next {
		if int(cur.id) == id {
			if prev == nil {
				l.head = cur.next
			} else {
				prev.next = cur.next
			}
			return true
		}
		prev = cur
	}
	return false
}

func (l *List) Splice(other *List) {
	if l.head == nil {
		l.head = other.head
	} else {
		t := l.head
		for t.next != nil {
			t = t.next
		}
		t.next = other.head
	}
	other.head = nil
}

func (l *List) String() string {
	s := ""
	for p := l.head; p != nil; p = p.next {
		s += fmt.Sprint(int(p.id), " ")
	}
	return s
}

func Len(head *umem.ucounted) (n int) {
	for p := head; p != nil; p = p.next {
		n++
	}
	return
}

func live() int { return int(umem.live_counted()) }

func main() {
	a, b := &List{}, &List{}
	for i := 1; i <= 5; i++ {
		a.Push(i)
	}
	fmt.Println(a, live())
	fmt.Println(a.Pop(), a.Pop(), a, live())
	a.Reverse()
	fmt.Println(a, live())
	fmt.Println(a.Remove(4), a.Remove(1), a.Remove(9), a, live())

	// Move nodes between lists, then splice one onto the other.
	a.Push(7)
	for a.head != nil {
		b.Push(a.Pop())
	}
	fmt.Println(a, "|", b, live())
	a.Push(10)
	a.Push(11)
	a.Splice(b)
	fmt.Println(a, "|", b, live(), Len(a.head))

	// Swap the tails of two nodes, and unlink with an intermediate that goes away.
	x, y := &umem.ucounted{id: 1, next: &umem.ucounted{id: 11}}, &umem.ucounted{id: 2, next: &umem.ucounted{id: 22}}
	x.next, y.next = y.next, x.next
	fmt.Println(x.id, x.next.id, y.id, y.next.id, live())
	m := &umem.ucounted{id: 3}
	other := &umem.ucounted{id: 4, next: &umem.ucounted{id: 5}}
	m.next = other.next
	other.next = nil
	other = nil
	fmt.Println(m.next.id, live())

	// A copy of a struct keeps what its pointers point to while the copy lives (in this block).
	{
		src := &umem.ucounted{id: 6, next: &umem.ucounted{id: 66}}
		cp := *src
		src = nil
		fmt.Println(cp.id, cp.next.id)
	}

	// Elements of a slice are structs too: a chain inside one allocation, and a slice element that
	// points to a node of its own.
	nodes := make([]umem.ucounted, 4)
	for i := 0; i < 3; i++ {
		nodes[i].id = C.int(i)
		nodes[i].next = &nodes[i+1]
	}
	nodes[3].next = &umem.ucounted{id: 33}
	fmt.Println(Len(&nodes[0]), nodes[0].next.next.next.next.id)

	// An array member and a nested struct member of a holder.
	h := &umem.uholder{}
	for i := 0; i < 3; i++ {
		h.slots[i] = &umem.unode{v: C.int(i + 1)}
	}
	h.in.p = h.slots[2]
	h.slots[2] = nil
	h.slots[1], h.slots[0] = h.slots[0], h.slots[1]
	fmt.Println(h.slots[0].v, h.slots[1].v, h.in.p.v, h.slots[2] == nil)

	// Two nodes that point at each other never die: break the cycle by hand.
	c1, c2 := &umem.ucounted{id: 100}, &umem.ucounted{id: 200}
	c1.next = c2
	c2.next = c1
	before := live()
	c1.next = nil
	c1, c2 = nil, nil
	fmt.Println(before-live() >= 2)
}

// Output:
// 5 4 3 2 1  5
// 5 4 3 2 1  3
// 1 2 3  3
// false true false 2 3  2
//  | 3 2 7  3
// 11 10 3 2 7  |  5 5
// 1 22 2 11 9
// 5 11
// 6 66
// 5 33
// 2 1 3 true
// true
