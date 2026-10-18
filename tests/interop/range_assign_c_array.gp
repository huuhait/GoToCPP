// phase: 04

// range with = into variables that exist: a gocpp array of C numbers into a C-typed variable (an
// internal error from the opaque checker once, from the 4.4 review), into a gocpp int index, and over
// a C++ container into C/C++ variables (a gocpp int index is an error there, see range_containers).
package main

import (
	"fmt"

	"<vector>"
)

func main() {
	arr := [3]C.int{10, 20, 30}
	var i int
	var x C.int
	for i, x = range arr {
		fmt.Println(i, int(x))
	}
	fmt.Println(i, int(x))

	v := std.vector[int]{7, 8}
	var k, e C.long
	for k, e = range v {
		fmt.Println(int(k), int(e))
	}
	for i = range arr {
	}
	fmt.Println(i)
}

// Output:
// 0 10
// 1 20
// 2 30
// 2 30
// 0 7
// 1 8
// 2
