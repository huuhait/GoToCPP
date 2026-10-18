// phase: 04

// A pointer stored into a member of a C struct (the members of C structs are raw pointers) keeps its
// object alive as long as the struct that holds it lives, as Go's pointer does. The struct must be
// owned by a *T variable (n := &T{...}, a parameter, a field, an element): n.next = head, n.in.p = x,
// n.slots[i] = x, n.data = unsafe.Pointer(x). A store replaces what the member kept before, and nil
// or a pointer from C drops it, so a loop that stores into one member holds one object. Under ASan,
// a store that kept nothing is a heap-use-after-free in every one of them.
package main

import (
	"fmt"
	"unsafe"

	"./umem.hpp"
)

type S struct{ node *umem.unode }

func push(head *umem.unode, v int) *umem.unode {
	n := &umem.unode{v: C.int(v)}
	n.next = head // head's owner goes out of scope with the call: n keeps it
	return n
}

func sum(head *umem.unode) (s int) {
	for p := head; p != nil; p = p.next {
		s += int(p.v)
	}
	return
}

func main() {
	// The classic list build: the old head is only n.next.
	var head *umem.unode
	for i := 1; i <= 4; i++ {
		n := &umem.unode{v: C.int(i)}
		n.next = head
		head = n
	}
	fmt.Println(sum(head))

	// A push helper, and a new node built over a list that C made.
	var h2 *umem.unode
	for i := 1; i <= 3; i++ {
		h2 = push(h2, i*10)
	}
	h3 := push(umem.build_list(2), 100)
	fmt.Println(sum(h2), sum(h3))
	umem.free_list(h3.next)

	// A temporary stored into a member, a pointer through a field and an element.
	n := &umem.unode{v: 1}
	n.next = &umem.unode{v: 2}
	s := S{node: &umem.unode{v: 7}}
	s.node.next = &umem.unode{v: 8}
	var ps []*umem.unode
	ps = append(ps, &umem.unode{v: 5})
	ps[0].next = &umem.unode{v: 6}
	fmt.Println(sum(n), sum(s.node), sum(ps[0]))

	// Many stores to one member hold one object; nil drops it. (live counts the objects.)
	c := &umem.ucounted{}
	for i := 0; i < 100; i++ {
		c.next = &umem.ucounted{id: C.int(i)}
	}
	fmt.Println(umem.live_counted(), int(c.next.id))
	c.next = nil
	fmt.Println(umem.live_counted())
	for i := 0; i < 3; i++ {
		d := &umem.ucounted{}
		d.next = c
		c = d
	}
	fmt.Println(umem.live_counted())
	c = nil
	fmt.Println(umem.live_counted())

	// A member of a struct member, an element of an array member, a void* member.
	h := &umem.uholder{}
	h.in.p = &umem.unode{v: 1}
	h.slots[1] = &umem.unode{v: 2}
	h.slots[2] = &umem.unode{v: 3}
	h.data = unsafe.Pointer(&umem.unode{v: 4})
	h.slots[2] = nil
	h.v = 9
	fmt.Println(int(h.in.p.v), int(h.slots[1].v), h.slots[2] == nil, int((*umem.unode)(h.data).v), int(h.v))

	// Two stores in one assignment, and a pointer from C (it owns nothing: C frees it).
	a, b := &umem.unode{v: 1}, &umem.unode{v: 2}
	x, y := &umem.unode{}, &umem.unode{}
	x.next, y.next = a, b
	a, b = nil, nil
	fmt.Println(int(x.next.v), int(y.next.v))
	l := umem.build_list(3)
	x.next = l
	fmt.Println(sum(x))
	umem.free_list(l)
	x.next = nil
}

// Output:
// 10
// 60 103
// 3 15 11
// 2 99
// 1
// 4
// 0
// 1 2 true 4 9
// 1 2
// 6
