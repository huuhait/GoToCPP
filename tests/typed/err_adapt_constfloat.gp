// phase: 12
// error: cannot use 1e60 \(untyped float constant 1e\+60\) as C.float value in argument to take_float \(overflows\)

// 1e60 does not fit a float.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	fmt.Println(adapt.take_float(1e60))
}
