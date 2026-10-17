// phase: 04

// A .h header with a .cpp file next to it, with the usual guards for C and C++ (step 4.16):
// the header has `#ifdef __cplusplus extern "C" {`, so gocpp doesn't wrap it (a header with its
// own __cplusplus guard is left as it is), and mathlib.cpp, which includes the same text, defines
// the functions with C linkage.
package main

import (
	"fmt"

	"./mathlib.h"
)

func main() {
	fmt.Println(mathlib.add(1, 2), mathlib.twice(21))
}

// Output:
// 3 42
