// phase: 12
// error: invalid array length consts\.NEG \(untyped int constant -7\)

// The macro is a real constant for go/types, which says what it says for a number, with the name
// the user wrote.
package main

import (
	"fmt"

	"./consts.hpp"
)

var a [consts.NEG]int

func main() {
	fmt.Println(len(a))
}
