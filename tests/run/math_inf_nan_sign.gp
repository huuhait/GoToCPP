// phase: 09
// math.Inf(sign), math.NaN() and the signs of zero: the bit patterns are Go's.

package main

import (
	"fmt"
	"math"
)

func main() {
	fmt.Printf("%016x %016x %016x %016x\n", math.Float64bits(math.Inf(1)), math.Float64bits(math.Inf(-1)), math.Float64bits(math.Inf(0)), math.Float64bits(math.NaN()))
	fmt.Println(math.IsInf(math.Inf(2), 1), math.IsInf(math.Inf(-3), -1), math.IsNaN(math.NaN()), math.NaN() == math.NaN())
	var z float64
	nz := math.Copysign(z, -1)
	fmt.Println(z, nz, 1/z, 1/nz, math.Signbit(nz), math.Signbit(z), nz == z)
	fmt.Printf("%016x %016x\n", math.Float64bits(nz), math.Float64bits(-z))
}

// Output:
// 7ff0000000000000 fff0000000000000 7ff0000000000000 7ff8000000000001
// true true true false
// 0 -0 +Inf -Inf true false true
// 8000000000000000 8000000000000000
