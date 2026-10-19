// The address of a temporary: a class result and a number result.
package main

import (
	"./shapes.hpp"
	"<cstdlib>"
)

func main() {
	_ = &shapes.make_counter()
	_ = &cstdlib.abs(1)
}
