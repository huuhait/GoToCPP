// phase: 04

package main

import (
	"fmt"

	"<algorithm>"
	"<map>"
	"<string>"
)

func main() {
	ages := std.map[std.string, int]{} // Go keyword "map" is allowed after "." (Phase 01 patch)
	ages["bob"] = 31                   // a C++ type keeps C++ rules: operator[] inserts
	for name, age := range ages {
		fmt.Println(name, age)
	}
	lo, hi := std.minmax(3, 1)
	fmt.Println(lo, hi)
}

// Output:
// bob 31
// 1 3
