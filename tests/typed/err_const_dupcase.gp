// phase: 12
// error: duplicate case consts\.ZERO

// A case that a macro makes equal to another is a duplicate for Go, as it is for C++.
package main

import (
	"fmt"

	"./consts.hpp"
)

func main() {
	x := 3
	switch x {
	case 0:
		fmt.Println("zero")
	case consts.ZERO:
		fmt.Println("also zero")
	}
}
