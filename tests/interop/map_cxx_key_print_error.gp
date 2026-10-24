// phase: 04

// fmt prints the keys of a map in order: a C++ class key without operator< has none.
package main

import (
	"fmt"

	"./map_cxx_key_class.hpp"
)

func main() {
	tags := map[mk.Tag]int{{id: 1}: 1}
	fmt.Println(len(tags))
	fmt.Println(tags) // ERROR "fmt prints the keys of a map in order, and a C\+\+ class key needs an operator<"
}
