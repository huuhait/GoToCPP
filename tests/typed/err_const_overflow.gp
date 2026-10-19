// phase: 12
// error: cannot use consts\.NEG \(untyped int constant -7\) as uint8 value in constant declaration \(overflows\)

// Go's constant rules apply to a constant declaration: -7 does not fit a uint8.
package main

import (
	"fmt"

	"./consts.hpp"
)

const b uint8 = consts.NEG

func main() {
	fmt.Println(b)
}
