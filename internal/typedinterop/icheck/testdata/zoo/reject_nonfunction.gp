// Calling a name that is a variable.
package main

import (
	"./shapes.hpp"
	"<cstdlib>"
)

func main() {
	_ = shapes.global_value()
	_ = cstdlib.abs(1)()
}
