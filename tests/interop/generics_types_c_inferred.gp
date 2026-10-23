// phase: 07

// A C type argument of a generic function that makes an instance of a generic type of it: the message, at the use of the function,
// says what is missing.
package main

import "fmt"

type Box[T any] struct{ v T }

func Wrap[T any](v T) Box[T] {
	return Box[T]{v}
}

func main() {
	fmt.Println(Wrap(C.int(1)).v) // ERROR "an instance of the generic type Box with a C/C\+\+ type argument of a generic function is not supported yet"
}
