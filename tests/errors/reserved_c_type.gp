// phase: 01

// A type, a function, a parameter or a range variable named C is an error
// too.
package main

type C int // ERROR "cannot declare C - C is reserved for C's built-in types"

func f(C int) {} // ERROR "cannot declare C - C is reserved for C's built-in types"

func main() {
	for C := range 3 { // ERROR "cannot declare C - C is reserved for C's built-in types"
		_ = C
	}
}
