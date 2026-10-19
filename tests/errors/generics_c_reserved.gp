// phase: 07

// Only the type parameters of funcs and of generic types may be named C: a variable, a parameter, a
// type or a function named C is still an error.
package main

type Box[C any] struct{ v C }

func (b Box[C]) Get() C { return b.v }

func main() {
	C := 1 // ERROR "cannot declare C - C is reserved for C's built-in types"
	_ = C
}

func F(C int) {} // ERROR "cannot declare C - C is reserved for C's built-in types"
