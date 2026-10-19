// phase: 12
// error: array length consts\.NEXT must be constant: the C\+\+ compiler cannot evaluate it at compile time

// A macro with a side effect is not a constant: the array length is an error at the .gp position,
// with the reason, before the C++ compile (opaque mode would leave it to the C++ compiler).
package main

import (
	"fmt"

	"./consts.hpp"
)

var a [consts.NEXT]int

func main() {
	fmt.Println(len(a))
}
