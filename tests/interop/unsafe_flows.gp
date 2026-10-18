// phase: 04

// A gocpp pointer to a C type (*lib.T in a variable, a parameter, a field, an element or a result) is
// a gp::cptr: it owns or aliases what it points to, as Go's pointer does, and a raw pointer from C
// is assigned to it. So the flows of Go code that make a pointer outlive its first variable work:
// a list built in a loop (each node dies with its iteration's variable), a pointer returned, stored
// in a struct or a slice, passed to a function that keeps it, and the address of a struct field or
// a slice element that stays valid after the struct or the slice is gone or grows. Under ASan, a
// pointer that kept nothing alive is a heap-use-after-free in every one of them.
package main

import (
	"fmt"
	"unsafe"

	"./umem.hpp"
)

type Wrapper struct{ h *umem.upoint }

type S struct{ n C.int }

type Holder struct{ p *umem.upoint }

func NewHolder(p *umem.upoint) *Holder { return &Holder{p} }

func newNode(v int) *umem.unode {
	n := &umem.unode{v: C.int(v)}
	return n // the object lives on in the result
}

func keep(p *C.int) *C.int { return p }

// field returns the address of a field of a gocpp struct: the pointer keeps the struct alive.
func field(s *S) *C.int { return &s.n }

func fresh() *umem.upoint {
	var p *umem.upoint = &umem.upoint{20, 21} // a variable declared with the pointer type
	return p
}

func sum(head *umem.unode) (s int) {
	for p := head; p != nil; p = p.next {
		s += int(p.v)
	}
	return
}

func main() {
	// The classic list build: each node lives on as the next node's next; the old head is kept by it.
	var head *umem.unode
	for i := 1; i <= 3; i++ {
		n := &umem.unode{v: C.int(i), next: head}
		head = n
	}
	fmt.Println(sum(head))
	var h2 *umem.unode
	for i := 1; i <= 3; i++ {
		h2 = &umem.unode{v: C.int(i * 10), next: h2}
	}
	fmt.Println(sum(h2), int(h2.v))

	// A pointer assigned to a variable of an outer block, a raw local, a copy of a raw local.
	var c *umem.upoint
	{
		p := &umem.upoint{1, 2}
		c = p
	}
	fmt.Println(int(c.x), int(c.y))
	n := &umem.unode{v: 5}
	var raw *umem.unode = n
	m := raw
	n = nil
	fmt.Println(int(m.v), int(raw.v))

	// Returned, stored in a struct, appended, passed to a function that keeps it.
	r := newNode(7)
	w := Wrapper{h: &umem.upoint{3, 4}}
	var ws []Wrapper
	ws = append(ws, Wrapper{h: &umem.upoint{5, 6}})
	var ps []*umem.upoint
	ps = append(ps, &umem.upoint{7, 8})
	hd := NewHolder(&umem.upoint{9, 10})
	fmt.Println(int(r.v), int(w.h.x), int(ws[0].h.y), int(ps[0].x), int(hd.p.y))
	umem.scale_point(hd.p, 2)
	fmt.Println(int(hd.p.x), int(umem.sum_points(ps[0], 1)))
	byName := map[string]*umem.upoint{"a": &umem.upoint{11, 12}}
	fmt.Println(int(byName["a"].y), byName["b"] == nil)

	// The address of a field or an element outlives the struct or slice, and its growth.
	s := &S{n: 4}
	q := keep(&s.n)
	s = nil
	fmt.Println(int(*q))
	cs := []C.int{1, 2, 3}
	var pr *C.int = &cs[1]
	cs = append(cs, 4, 5, 6, 7)
	*pr = 20
	fmt.Println(int(*pr), int(cs[1]), len(cs))

	// The address of a field as a result, and &T{} in a variable declared with its pointer type.
	fp := field(&S{n: 31})
	fq := fresh()
	fmt.Println(int(*fp), int(fq.x), int(fq.y))

	// Comparisons, a switch, unsafe.Pointer and back, a pointer to a pointer from C.
	list := umem.build_list(3)
	top := &umem.unode{v: 9, next: list}
	same := list
	fmt.Println(top.next == same, top != same, same == top.next, top == nil, nil == top)
	switch top {
	case nil:
		fmt.Println("nil")
	default:
		fmt.Println("non-nil")
	}
	up := unsafe.Pointer(top)
	top = nil
	fmt.Println(int((*umem.unode)(up).v), unsafe.Pointer(same) != nil)
	umem.free_list(list)
}

// Output:
// 6
// 60 30
// 1 2
// 5 5
// 7 3 6 7 10
// 18 15
// 12 true
// 4
// 20 2 7
// 31 20 21
// true true true false false
// non-nil
// 9 true
