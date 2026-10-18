// phase: 05

// A store of a gocpp pointer into a member promoted from an embedded C struct keeps the object
// alive like the store into the explicit path (n.unode.next = p, step 4.10): through a gocpp
// pointer to the outer struct, also for a member of a promoted C struct and an element of its array.
package main

import (
	"fmt"

	"./umem.hpp"
)

type N struct {
	umem.unode
	tag int
}

type H struct{ umem.uholder }

func main() {
	n := &N{tag: 1}
	other := &umem.unode{v: 5}
	n.next = other // promoted: the same as n.unode.next = other
	other = nil
	fmt.Println(int(n.next.v), n.tag)
	for i := 0; i < 3; i++ {
		n.next = &umem.unode{v: C.int(10 + i)}
	}
	fmt.Println(int(n.next.v), int(n.unode.next.v))
	hh := &H{}
	hh.in.p = &umem.unode{v: 3}
	hh.slots[1] = &umem.unode{v: 4}
	fmt.Println(int(hh.in.p.v), int(hh.slots[1].v))
}

// Output:
// 5 1
// 12 12
// 3 4
