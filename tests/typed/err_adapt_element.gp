// phase: 12
// error: C\+\+:|cannot use

// Only the address of a variable or a field is adapted. The address of an element is the start of
// an array to a C parameter, so it stays C++'s: here it is an error.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	values := []int{1, 2, 3}
	fmt.Println(adapt.bump(&values[1]))
}
