// phase: 07

// A constraint with a method, satisfied by a C++ type: go/types cannot check it, the C++ compiler does, and x.size() is a member call.
package main

import (
	"fmt"

	"<string>"
	"<vector>"
)

func Size[T interface{ size() C.size_t }](x T) int { return int(x.size()) }

func Total[T interface{ size() C.size_t }](xs ...T) int {
	n := 0
	for _, x := range xs {
		n += int(x.size())
	}
	return n
}

func Empty[T interface{ empty() C.bool }](x T) bool { return bool(x.empty()) }

func main() {
	v := std.vector[int]{1, 2, 3}
	s := std.string("hello")
	fmt.Println(Size(v), Size(s), Total(v, v), Total(s))
	fmt.Println(Empty(v), Empty(std.string("")))
}

// Output:
// 3 5 6 5
// false true
