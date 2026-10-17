// phase: 04
// ExitCode: 2
// Stderr: ^panic: runtime error: index out of range \[2\] with length 2\n

// A body that shrinks a vector during a range over it (step 4.7): the size was read once, so
// the walk reaches an index past the new end, and that panics with Go's index error instead of
// reading a destroyed element (Go itself reads the old array; a C++ vector has no such array).
package main

import (
	"fmt"

	"<vector>"
)

func main() {
	var v std.vector[C.int]
	v.push_back(1)
	v.push_back(2)
	v.push_back(3)
	v.push_back(4)
	for i, x := range v {
		fmt.Println(int(i), int(x))
		v.pop_back()
	}
}

// Output:
// 0 1
// 1 2
