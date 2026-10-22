// phase: 12
// error: C\+\+:|cannot use

// A parameter written int values[2] is written twice by the callee: the address of one Go
// variable is not adapted for it (the callee would write past the temporary).
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var count int64 = 3
	adapt.drag_pair(&count)
	fmt.Println(count)
}
