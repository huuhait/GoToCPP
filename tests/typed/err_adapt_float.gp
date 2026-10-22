// phase: 12
// error: C converts neither way through a pointer

// A pointer to an integer is not a pointer to a double: the bytes would be read as the wrong type.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	n := 3
	adapt.scale(&n)
	fmt.Println(n)
}
