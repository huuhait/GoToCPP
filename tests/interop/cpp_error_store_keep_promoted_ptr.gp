// phase: 05

// The same through an embedded pointer *N: the owner is m.N, which has no expression to name, so the error says to write the path.
package main

import "./umem.hpp"

type N struct {
	umem.unode
	tag int
}

type M struct {
	*N
	k int
}

func main() {
	m := M{&N{}, 2}
	m.next = &umem.unode{v: 2} // ERROR "stored into m\.next.*write the path of the embedded fields, m\.N\.unode\.next"
	_ = m
}
