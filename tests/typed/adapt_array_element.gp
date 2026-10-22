// phase: 12
// opaque: reject

// A parameter written as an array takes several elements, so the address of one variable is not adapted
// for it (err_adapt_array_*); the address of the first element of an array or a slice is the pointer it
// wants, and the callee reads and writes all of the elements.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	color := [3]float32{0.25, 0.5, 0.25}
	fmt.Println(adapt.mix_color("x", &color[0]))

	pair := [2]int32{4, 9}
	adapt.drag_pair(&pair[0])
	fmt.Println(pair[0], pair[1])

	values := []int32{1, 2, 3}
	fmt.Println(adapt.sum_three(&values[0]))

	var stored int64
	fmt.Println(adapt.store_five(&stored), stored)
}

// Output:
// 1
// 5 10
// 6
// 0 5
