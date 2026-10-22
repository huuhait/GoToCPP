// phase: 12
// error: cannot use 1 << 40 \(untyped int constant 1099511627776\) as C.int value in argument to take_int \(overflows\)

// 1<<40 does not fit an int; take_longlong takes it.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	fmt.Println(adapt.take_int(1 << 40))
}
