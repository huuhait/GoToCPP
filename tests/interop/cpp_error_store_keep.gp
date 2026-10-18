// phase: 04

// n.next.next = p stores through the raw pointer n.next: the struct there is not n's, so n
// can't keep p's object alive. The C++ compiler tells (the type of the member is C's).
package main

import "./umem.hpp"

func main() {
	head := &umem.unode{v: 1}
	tail := &umem.unode{v: 2}
	head.next = tail
	head.next.next = tail // ERROR "C\+\+: cannot keep the object alive: the struct is reached through a raw C pointer"
	h := &umem.uholder{}
	h.in.p = tail
	h.slots[0] = tail
	h.in.p.next = head // ERROR "C\+\+: cannot keep the object alive: the struct is reached through a raw C pointer"
}
