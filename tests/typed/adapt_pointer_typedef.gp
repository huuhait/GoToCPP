// phase: 12
// opaque: reject

// A parameter written with the typedef of a pointer (typedef int* IntPtr) is a pointer: the address of
// a variable is adapted for it as for int*. The address of the first element is the pointer that an
// array typedef takes.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var counter int64 = 10
	fmt.Println(adapt.bump_typedef(&counter), counter)

	values := [3]int32{1, 2, 3}
	fmt.Println(adapt.tdef(&values[0]), values[2])

	var rack adapt.Rack
	fmt.Println(rack.total(&values[0]))
}

// Output:
// 17 17
// 6 40
// 43
