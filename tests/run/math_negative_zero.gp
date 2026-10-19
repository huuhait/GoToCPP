// phase: 09
// Negative zero: -x of a variable, 0 * -1, x - x, Copysign, Signbit, printing, and division by it.

package main

import (
	"fmt"
	"math"
)


func main() {

	var z float64
	var f32 float32
	nz := -z
	fmt.Println(nz, -f32, z*-1, 0*nz, nz+nz, nz-z, z-z, nz+z, 1/nz, 1/z)
	fmt.Printf("%v %g %e %f %.2f %+v %08.3f %x\n", nz, nz, nz, nz, nz, nz, nz, nz)
	fmt.Println(math.Signbit(nz), math.Signbit(z*-1), math.Signbit(float64(-f32)), math.Copysign(3, nz), math.Copysign(nz, 1))
	fmt.Printf("%016x %016x %016x\n", math.Float64bits(nz), math.Float64bits(nz*nz), math.Float64bits(math.Abs(nz)))
	fmt.Println(math.Sqrt(nz), math.Floor(nz), math.Ceil(-0.3), math.Trunc(-0.3), math.Round(-0.3), math.RoundToEven(-0.3), math.Mod(nz, 3), math.Max(nz, z), math.Min(nz, z))

}

// Output:
// -0 -0 -0 -0 -0 -0 0 0 -Inf +Inf
// -0 -0 -0.000000e+00 -0.000000 -0.00 -0 -000.000 -0x0p+00
// true true true -3 0
// 8000000000000000 0000000000000000 0000000000000000
// -0 -0 -0 -0 -0 -0 -0 0 -0
