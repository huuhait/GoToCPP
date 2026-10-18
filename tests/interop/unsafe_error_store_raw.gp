// phase: 04

// A gocpp pointer stored into a member of a C struct is kept alive by the struct only when a *T
// variable owns the struct (n := &T{...}). Where none does, nothing could keep the object alive:
// a pointer from C.
package main

import "./umem.hpp"

func main() {
	head := &umem.unode{v: 1}
	raw := umem.build_list(2)
	raw.next = head // ERROR "cannot keep the object alive: head is stored into raw\.next, which no \*T variable of the program owns"
	umem.free_list(raw)
}
