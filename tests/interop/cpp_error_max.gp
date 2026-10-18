// phase: 04

// The untyped-constant rule (docs/spec/interop.md, "Untyped constants in C/C++ calls"): 0 is a C int,
// n is an int64_t, and std::max deduces one type from both, so the call does not compile. The fix is
// a typed constant, std.max(n, int(0)).
package main

import (
	"fmt"

	"<algorithm>"
)

func main() {
	n := 3
	fmt.Println(int(std.max(n, 0))) // ERROR "C\+\+: no matching function for call to 'max"
	fmt.Println(int(std.max(n, int(0))))
}
