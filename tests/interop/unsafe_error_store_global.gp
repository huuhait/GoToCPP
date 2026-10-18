// phase: 04

// A gocpp pointer stored into a member of a C struct is kept alive by the struct only when a *T
// variable owns the struct (n := &T{...}). Where none does, nothing could keep the object alive:
// a C global.
package main

import "./umem.hpp"

func main() {
	head := &umem.unode{v: 1}
	umem.kept_node = head // ERROR "cannot keep the object alive: head is stored into umem\.kept_node, which no \*T variable of the program owns"
}
