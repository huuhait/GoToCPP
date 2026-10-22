// phase: 12
// error: C\+\+:|cannot use

// A parameter written with the typedef of an array (typedef int IVec[3]) is an array although the
// compiler gives it the type int*: the address of one variable is not adapted for it.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var count int64 = 3
	fmt.Println(adapt.tdef(&count))
}
