// phase: 05

// The store into a member promoted from an embedded C struct that is reached through a value (no gocpp
// pointer owns the struct) is the error of step 4.10, with the path of the embedded fields to write.
package main

import "./umem.hpp"

type N struct {
	umem.unode
	tag int
}

func main() {
	var v N
	v.next = &umem.unode{v: 1} // ERROR "cannot keep the object alive: &umem\.unode\{…\} is stored into v\.next.*write the path of the embedded fields, v\.unode\.next"
	_ = v
}
