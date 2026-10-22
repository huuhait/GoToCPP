// phase: 12
// error: C\+\+:|cannot use

// A typedef of a two-dimensional array (typedef float Mat4[4][4]) is an array of arrays: nothing is
// adapted for it.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var cell float64 = 1
	fmt.Println(adapt.tdef_matrix(&cell))
}
