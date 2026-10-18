// phase: 06

// A C library from a cxx recipe (Phase 06 step 6.1d) that two packages import and the main package
// does not: the program builds it once (the union of the libraries of all the packages) and links
// it, with the public flags of the recipe in the compiles of both packages.
package main

import (
	"fmt"

	"example.com/jsonapp/jsonx"
	"example.com/jsonapp/stats"
)

func main() {
	fmt.Println(jsonx.Sum("[1, 2, 3.5]"), stats.Len("[1, 2, 3.5]"), jsonx.MaxDepth())
	fmt.Println(jsonx.Sum("[1, 2,"), stats.Len("["))
}

// Output:
// 6.5 3 16
// -1 -1
