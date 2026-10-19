// phase: 07

// The same message for a composite literal with a C++ type argument.
package main

import (
	"fmt"

	"<vector>"
)

type Stack[T any] struct{ items []T }

func main() {
	s := Stack[std.vector[int]]{} // ERROR "an instance of a generic type with a C/C\+\+ type argument \(Stack\[C.std.vector\[int\]\]\) is not supported yet"
	fmt.Println(s)
}
