// phase: 04

// Functions of <cmath> and <math.h>, with C/C++ values converted to gocpp types to print them,
// and C/C++ values as variables, parameters and results.
package main

import (
	"fmt"

	"<cmath>"
	m "<math.h>"
)

func hyp(a, b C.double) C.double {
	return std.hypot(a, b)
}

func main() {
	fmt.Println(float64(cmath.sqrt(16.0)), float64(cmath.floor(-2.5)), float64(m.fabs(-3.25)))
	x := cmath.trunc(3.7)
	var y C.double = cmath.ceil(1.2)
	fmt.Println(float64(x), float64(y), int(cmath.lround(2.5)))
	fmt.Println(float64(hyp(3, 4)), float64(std.fmax(1.5, float64(2))))
}

// Output:
// 4 -3 3.25
// 3 2 3
// 5 2
