// phase: 04

// A gocpp pointer stored into a member of a C struct is kept alive by the struct only when a *T
// variable owns the struct (n := &T{...}). Where none does, nothing could keep the object alive:
// a C/C++ local value.
package main

import "./umem.hpp"

func main() {
	head := &umem.unode{v: 1}
	var local umem.unode
	local.next = nil
	local.next = umem.build_list(1) // a pointer from C needs no owner
	local.next = head // ERROR "cannot keep the object alive: head is stored into local\.next, which no \*T variable of the program owns"
}
