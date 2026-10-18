// phase: 04

// A pointer read from a member of a C struct that gocpp made owns the struct it points to, wherever
// it goes: appended to a slice, a map key, a field, the operand of unsafe.Pointer, a variable that
// then stores through it (t := l.next; t.next = x), and the operands of comparisons. Under ASan, a
// read that owned nothing is a heap-use-after-free after the struct that held it goes.
package main

import (
	"fmt"
	"unsafe"

	"./umem.hpp"
)

type Holder struct {
	saved *umem.ucounted
	all   []*umem.ucounted
	byId  map[*umem.ucounted]int
}

func build(n int) *umem.ucounted {
	var h *umem.ucounted
	for i := n; i >= 1; i-- {
		h = &umem.ucounted{id: C.int(i), next: h}
	}
	return h
}

func main() {
	h := &Holder{byId: map[*umem.ucounted]int{}}
	head := build(4)
	cur := head
	for cur != nil {
		h.all = append(h.all, cur.next)  // raw read into append
		h.byId[cur.next] = int(cur.id)    // raw read as a map key
		cur = cur.next
	}
	h.saved = head.next.next
	head = nil
	fmt.Println(len(h.all), len(h.byId), h.saved.id, h.all[1].id, umem.live_counted())

	// A store through a variable that took the member: l.next.next = x written in two steps.
	l := build(3)
	t := l.next
	t.next = &umem.ucounted{id: 9}
	fmt.Println(l.next.next.id, umem.live_counted())

	// unsafe.Pointer of a read keeps the owner.
	l2 := build(2)
	up := unsafe.Pointer(l2.next)
	l2 = nil
	fmt.Println((*umem.ucounted)(up).id)

	// Reads compare by address.
	a := build(2)
	fmt.Println(a.next == a.next, a.next != nil, a.next.next == nil)
}

// Output:
// 4 4 3 3 3
// 9 7
// 2
// true true true
