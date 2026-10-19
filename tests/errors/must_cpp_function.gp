// phase: 05

// ! works on Go functions whose last result is error. A C or C++ function does not return a Go
// error, and the compiler says so.
package main

import (
	"fmt"

	"<numeric>"
	"<vector>"
)

func main() {
	v := std.vector[int]{1, 2}
	n := std.accumulate!(v.begin(), v.end(), 0) // ERROR "std.accumulate!\(...\): ! is not supported on C and C\+\+ functions yet"
	fmt.Println(n)
}
