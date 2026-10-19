// phase: 07

// A generic type takes Go types as arguments: C++ types (std.string, std.vector[int]) are not supported as arguments of a type yet, and the message says so.
package main

import (
	"fmt"

	"<string>"
	"<vector>"
)

type Box[T any] struct{ v T }

func (b Box[T]) Get() T { return b.v }

func main() {
	var b Box[std.string] // ERROR "an instance of a generic type with a C/C\+\+ type argument \(Box\[C.std.string\]\) is not supported yet"
	fmt.Println(b)
}
