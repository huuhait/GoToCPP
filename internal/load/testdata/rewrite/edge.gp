package main

import (
	"fmt"
	"slices"

	"<cstdio>"
	"<vector>"
	"<string.h>" // its file name is Go's `string`, so it gets no package name
)

// E1: C macro used where Go needs a compile-time constant
const N = cstdio.BUFSIZ

var buf [N]byte

// E2: embedding a C++ type and calling a promoted method
type MyVec struct {
	std.vector
}

type Shape interface{ Area() float64 }

func main() {
	var mv MyVec
	mv.push_back(1) // promoted from the embedded C++ type?

	// E3: generic stdlib call with an opaque argument
	v := std.vector[int]{3, 1, 2}
	fmt.Println(slices.Contains(v, 3))

	// E4: type switch / assertion on an opaque value
	x := cstdio.get_thing()
	switch x.(type) {
	case int:
	}
	var sh Shape = x // opaque value stored in a gocpp interface

	// E5: real gocpp errors must still be reported
	var bad int = "nope"
	undefinedThing()
	fmt.Println(len(buf), sh, bad)

	// E6: typos in package or namespace names must still be reported
	w := stdd.vector[int]{}
	fmtt.Println(w)

	// E7: the unnamed <string.h>: using it by its file name gets a hint
	fmt.Println(string.strlen("x"))
}
