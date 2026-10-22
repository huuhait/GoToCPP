// phase: 12
// error: cannot use 2.5 \(untyped float constant\) as C.int value in argument to take_int \(truncated\)

// 2.5 is not a whole number: Go refuses to cut it.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	fmt.Println(adapt.take_int(2.5))
}
