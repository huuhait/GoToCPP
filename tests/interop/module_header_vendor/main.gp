// phase: 04

// A header of a vendored module (Phase 06 step 6.1b): vendor/modules.txt says that example.org/mathlib is
// in vendor/example.org/mathlib, and the project has no replace line and downloads nothing.
package main

import (
	"fmt"

	"example.org/mathlib/mathlib.hpp"
)

func main() {
	fmt.Println(mathlib.ml_square(5), mathlib.ml_scaled(5))
}

// Output:
// 25 15
