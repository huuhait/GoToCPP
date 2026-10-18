// phase: 04

// A gocpp pointer stored into a member of a C struct is kept alive by the struct only when a *T
// variable owns the struct (n := &T{...}). Where none does, nothing could keep the object alive:
// a temporary stored into a struct that C owns.
package main

import "./umem.hpp"

func main() {
	head := &umem.unode{v: 1}
	_ = head
	raw := umem.build_list(2)
	raw.next = &umem.unode{v: 3} // ERROR "cannot keep the object alive: &umem\.unode\{…\} is stored into raw\.next"
	umem.free_list(raw)
}
