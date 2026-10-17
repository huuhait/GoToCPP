// phase: 04

// A helper header next to the .gp file is a package (step 4.16, README §2.10): there is no cgo
// preamble. mathx.h is "package mathx" and has inline functions, a template, a constant, a class,
// an enum and a macro; the header includes another local header (units.h).
package main

import (
	"fmt"

	"./mathx.h"
)

func main() {
	fmt.Println(mathx.hypot3(1, 2, 2))
	fmt.Println(mathx.clamp_to(150, 0, mathx.mx_limit), mathx.clamp_to(2.5, 0.0, 1.0))
	var c mathx.mx_counter
	c.add(40)
	c.add(2)
	fmt.Println(c.get(), mathx.MX_CM_PER_M, int(mathx.MX_SLOW))
	fmt.Println(string(mathx.mx_describe(1.5)))
	var m mathx.mx_mode = mathx.MX_FAST
	fmt.Println(int(m))
}

// Output:
// 3
// 100 1
// 42 100 5
// 150 cm
// 0
