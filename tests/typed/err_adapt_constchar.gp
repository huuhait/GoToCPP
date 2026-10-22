// phase: 12
// error: cannot use 200 \(untyped int constant\) as C.char value in argument to take_char \(overflows\)

// 200 does not fit a char (C.char is int8): Go's rule, where C++ would wrap it to -56.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	fmt.Println(adapt.take_char(200))
}
