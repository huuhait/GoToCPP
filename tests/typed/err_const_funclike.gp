// phase: 12
// error: consts\.TWICE is not constant: it is a macro that does not expand to an expression

// A function-like macro has no value without its arguments.
package main

import (
	"fmt"

	"./consts.hpp"
)

const n = consts.TWICE

func main() {
	fmt.Println(n)
}
