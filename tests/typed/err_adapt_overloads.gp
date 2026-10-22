// phase: 12
// error: C\+\+:|cannot use|no matching|ambiguous

// Two overloads that adapt differently (int* and long long*) are not told apart by the compiler:
// the call is left to C++, which refuses an int64 variable's address.
package main

import (
	"fmt"

	"./adapt.hpp"
)

func main() {
	var n int64 = 5
	fmt.Println(adapt.pick(&n))
}
