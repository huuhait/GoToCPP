// phase: 01

// C is reserved for C's built-in types (Phase 01 README §2.3): no
// declaration in a gocpp package can have that name.
package main

var C = 1 // ERROR "cannot declare C - C is reserved for C's built-in types"

func main() {
	C := 2 // ERROR "cannot declare C - C is reserved for C's built-in types"
	_ = C
}
