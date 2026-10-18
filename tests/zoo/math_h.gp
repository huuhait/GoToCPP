// phase: 04

// <math.h>: functions with double and float parameters, the M_PI and INFINITY macros, and
// frexp with an out parameter.
package main

import (
	"fmt"

	"<math.h>"
)

func main() {
	fmt.Println(float64(math.sqrt(16)), float64(math.pow(2, 10)), float64(math.floor(-1.5)))
	fmt.Printf("%.4f %.4f\n", float64(math.sin(math.M_PI/2)), float64(math.cos(0)))
	var e C.int
	m := math.frexp(8, &e)
	fmt.Println(float64(m), int(e))
	fmt.Println(float32(math.sqrtf(9)), bool(math.isinf(math.INFINITY) != 0), float64(math.fabs(-0.25)))
}

// Output:
// 4 1024 -2
// 1.0000 1.0000
// 0.5 4
// 3 true 0.25
