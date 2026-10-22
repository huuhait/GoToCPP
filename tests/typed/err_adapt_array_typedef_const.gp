// phase: 12
// error: C\+\+:|cannot use

// The same with const in front of the typedef (const IVec): the callee reads three values.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var count int64 = 3
	fmt.Println(adapt.tdef_c(&count))
}
