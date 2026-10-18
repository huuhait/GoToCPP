// phase: 06

// The C and C++ files of packages other than main: csrc/csrc.c (with the //gocpp:cflags and
// //gocpp:link of its package, which also reach the link of the program) and cppsrc/cppsrc.cpp are
// compiled with their own package's flags and linked into the program.
package main

import (
	"fmt"

	"example.com/csrc/cppsrc"
	"example.com/csrc/csrc"
)

func main() {
	fmt.Println(csrc.Twice(20), csrc.Root(81), cppsrc.Triple(7))
}

// Output:
// 41 9 21
