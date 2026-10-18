// phase: 04

// Pointers that own what they point to (step 4.10): &T{...} of a C type, &s.f of a gocpp struct
// and &xs[i] of a gocpp slice or array are gocpp pointers that keep the object alive, as in Go,
// and convert to the raw T* that C takes (borrowed). A raw pointer from C is assigned to them, so
// a list walk starts at a node of &T{...} and goes on over the nodes that C built (and the other
// way round); and unsafe.Pointer(&xs[0]) keeps the slice's array alive. Under ASan, a pointer
// that owned nothing is a heap-use-after-free here.
package main

import (
	"fmt"
	"unsafe"

	"<mutex>"

	"./umem.hpp"
)

type S struct {
	n    C.int
	vals [3]C.int
}

func newS() *S {
	s := &S{}
	s.n = 5
	return s
}

// firstOf returns the elements of a slice that a function made: the pointer to the first one is
// all that is left of it.
func firstOf() unsafe.Pointer {
	xs := []C.int{7, 8, 9}
	return unsafe.Pointer(&xs[0])
}

func main() {
	// A list walk over nodes that C built, and over nodes of &T{...} that point to them.
	head := umem.build_list(4)
	sum := 0
	for p := head; p != nil; p = p.next {
		sum += int(p.v)
	}
	fmt.Println("c list", sum, int(umem.sum_list(head)))

	b := &umem.unode{v: 20, next: head}
	a := &umem.unode{v: 10, next: b}
	var last *umem.unode // a raw local pointer
	for p := a; p != nil; p = p.next {
		last = p
		sum += int(p.v)
	}
	fmt.Println("mixed list", sum, int(last.v), int(umem.sum_list(a)))
	umem.keep_node(b) // C may keep it as long as b is alive
	fmt.Println(int(umem.kept_value()), a.next == b, a != nil, a.next.next == head)
	b.v = 21
	fmt.Println(int(umem.kept_value()))
	umem.free_list(head)

	// The address of a field keeps the struct alive.
	pn := &newS().n
	*pn += 1
	fmt.Println(int(*pn))
	s := newS()
	pv := &s.vals[2]
	*pv = 42
	s = nil
	fmt.Println(int(*pv), int(umem.sum_ints(pv, 1)))

	// The address of an element keeps the slice's array alive, also after the slice grows or is gone.
	xs := []C.int{1, 2, 3}
	p := &xs[0]
	xs = append(xs, 4, 5, 6, 7)
	*p = 10
	fmt.Println(int(*p), len(xs), int(xs[0]))
	q := unsafe.Pointer(&xs[1])
	xs = nil
	fmt.Println(int(*(*C.int)(q)))
	r := firstOf()
	fmt.Println(int(*(*C.int)(r)), int(*(*C.int)(unsafe.Add(r, 8))))
	up := unsafe.Pointer(&umem.upoint{3, 4})
	fmt.Println(int((*umem.upoint)(up).y))

	// A local array of C elements (boxed: its address is taken).
	var arr [4]C.int
	pa := &arr[2]
	*pa = 6
	fmt.Println(int(arr[2]), int(umem.sum_ints(&arr[0], 4)))

	// A C++ object that can't be moved or copied is built in place.
	m := &std.mutex{}
	m.lock()
	m.unlock()
	fmt.Println(m.try_lock(), m != nil)
	m.unlock()
}

// Output:
// c list 10 10
// mixed list 50 4 40
// 20 true true true
// 21
// 6
// 42 42
// 10 7 1
// 2
// 7 9
// 4
// 6 6
// true true
