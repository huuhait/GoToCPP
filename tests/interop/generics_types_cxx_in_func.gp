// phase: 07

// A generic function that makes an instance of a generic type of its type parameter works for Go types; for a C++ type argument the message, at the use of the
// function, says what is missing.
package main

import (
	"fmt"

	"<string>"
)

type Box[T any] struct{ v T }

func Local[T any](v T) int {
	b := Box[T]{v}
	_ = b
	return 1
}

func main() {
	fmt.Println(Local(1))
	fmt.Println(Local(std.string("x"))) // ERROR "an instance of the generic type Box with a C/C\+\+ type argument of a generic function is not supported yet"
}
