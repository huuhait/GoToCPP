// phase: 07

// What the C++ compiler says about the body of a generic function instantiated with a C++ type is the user's error at the line of the body (fmt cannot print a std::vector), with the call as "from here".
package main

import (
	"fmt"

	"<vector>"
)

func Printv[T any](x T) { fmt.Println(x) } // ERROR "C\+\+: fmt can't print a C\+\+ type"

func main() {
	v := std.vector[int]{1, 2}
	Printv(v)
}
