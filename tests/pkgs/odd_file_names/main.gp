// phase: 06

// File names that need care: spaces, a quote, a dollar, a dash and Unicode in the names of .gp files (their units are lib/<name>.gp.cpp).

package main

import (
	"fmt"

	"example.com/oddfiles/lib"
)

func main() { fmt.Println(lib.A(), lib.B(), lib.C3(), lib.D(), lib.E()) }

// Output:
// 1 2 3 4 5
