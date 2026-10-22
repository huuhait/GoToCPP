// phase: 12
// error: cannot use -1 \(untyped int constant\) as C.uint value in argument to take_uint \(overflows\)

// -1 does not fit an unsigned int: write 0xFFFFFFFF or C.uint(math.MaxUint32) for the all-ones value.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	fmt.Println(adapt.take_uint(-1))
}
