// phase: 05

// == of a gocpp struct compares its C/C++ fields with C++'s ==: a C++ type without one is an error
// at the comparison.
package main

import (
	"fmt"

	"./map_cxx_key_class.hpp"
)

type W struct {
	v mk.NoEq
	n int
}

func main() {
	a, b := W{n: 1}, W{n: 1}
	fmt.Println(a == b) // ERROR "== compares the C/C\+\+ parts of this value with C\+\+'s ==, which a C/C\+\+ type in it doesn't have"
}
