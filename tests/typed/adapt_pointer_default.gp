// phase: 12
// opaque: reject

// A pointer parameter with a default argument is still one pointer: the address of a variable is
// adapted for it, whatever the spaces around the "=", in a function and in a method.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var counter int64 = 10
	fmt.Println(adapt.count_up("a", &counter), counter)
	fmt.Println(adapt.count_up("b"))

	var palette adapt.Palette
	var total int64 = 1
	fmt.Println(palette.count(&total), total)
}

// Output:
// 11 11
// 0
// 1 6
