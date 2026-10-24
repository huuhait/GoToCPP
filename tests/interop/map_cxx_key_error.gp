// phase: 04

// A C++ type without std::hash (std.vector[int]) is no key of a gocpp map: the build says so at the
// map's line.
package main

import (
	"fmt"

	"<vector>"
)

func main() {
	k := map[std.vector[int]]int{} // ERROR "this C/C\+\+ type can't be the key of a gocpp map"
	fmt.Println(len(k))
}
