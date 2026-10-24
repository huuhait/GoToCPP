// phase: 04

// A gocpp map with a C++ key (map[std.string]int): gp::map hashes it with std::hash and compares it
// with its ==, in literals, make, var declarations, index, comma-ok, delete and range.
package main

import (
	"fmt"

	"<string>"
)

func main() {
	k := map[std.string]int{"a": 1}
	k[std.string("b")] = 2
	v, ok := k[std.string("a")]
	fmt.Println(len(k), v, ok, k[std.string("zz")])
	delete(k, std.string("a"))
	for key, val := range k {
		fmt.Println(key, val)
	}
	var m map[std.string][]int
	fmt.Println(m == nil, len(m))
	m = make(map[std.string][]int)
	m[std.string("x")] = append(m[std.string("x")], 3)
	m[std.string("x")] = append(m[std.string("x")], 4)
	fmt.Println(len(m), len(m[std.string("x")]))
}

// Output:
// 2 1 true 0
// b 2
// true 0
// 1 2
