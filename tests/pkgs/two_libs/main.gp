// phase: 06

// Two C libraries from cxx recipes in packages other than the first and the last of the program (a
// has none; b: minijson; c: minijson and tinyc, whose recipe has a public define that its header
// insists on): the program builds the union, each library once, and links both. The main package
// imports neither header.
package main

import (
	"fmt"

	"example.com/twolibs/a"
	"example.com/twolibs/b"
	"example.com/twolibs/c"
)

func main() {
	fmt.Println(a.Twice(3), b.Sum("[1, 2, 3.5]"), c.Both("[1, 1]"))
}

// Output:
// 6 6.5 44
