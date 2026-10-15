// phase: 04

// Printing a C++ value whose type has no std::formatter and no operator<< (also as a field of
// a gocpp struct), and a C++ object with the builtin println (Go's println prints only basic
// values, also when they are the results of a call, println(pq())): errors at the .gp lines,
// with the runtime's messages.
package main

import (
	"fmt"

	"./cxxvals.hpp"
)

type Box struct {
	o cxxvals.Opaque
	n int
}

// pq's first result has another type than the println above: C++ reports a static_assert once
// per type.
func pq() (cxxvals.Money, int) {
	return cxxvals.make_money(150), 3
}

func main() {
	var b Box
	fmt.Println(cxxvals.make_opaque()) // ERROR "C\+\+: fmt can't print a C\+\+ type in this value: it has no std::formatter and no operator<<"
	fmt.Printf("%v\n", b)              // ERROR "C\+\+: fmt can't print a C\+\+ type in this value"
	println(cxxvals.make_point(1, 2))  // ERROR "C\+\+: print and println can't print a C\+\+ object: use fmt.Println"
	println(pq())                      // ERROR "C\+\+: print and println can't print a C\+\+ object"
	fmt.Println(pq())
	fmt.Println(cxxvals.make_point(1, 2), b.n)
}
