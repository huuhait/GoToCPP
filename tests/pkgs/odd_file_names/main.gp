// phase: 06

// File names that need care: spaces, a hash, a dollar, a dash and Unicode in the names of .gp files (their units are lib/<name>.gp.cpp). An apostrophe is not allowed in a file of a Go module zip (go install), so that name is made at run time by cmd/gocpp TestApostropheInFileName.

package main

import (
	"fmt"

	"example.com/oddfiles/lib"
)

func main() { fmt.Println(lib.A(), lib.B(), lib.C3(), lib.D(), lib.E()) }

// Output:
// 1 2 3 4 5
