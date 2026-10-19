// An assignment to a C++ value that is not a variable.
package main

import (
	"./shapes.hpp"
	"<cstdlib>"
)

func main() {
	cstdlib.abs(1) = 3
	shapes.make_counter().n = 4
	shapes.make_counter().n++
	shapes.cpoint().x = 5
}
