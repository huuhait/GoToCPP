// phase: 09
// The constants of package math converted to float32: rounded to nearest, as Go does.

package main

import (
	"fmt"
	"math"
)


func main() {

	for _, f := range []float32{math.E, math.Pi, math.Phi, math.Sqrt2, math.Ln2, math.Ln10, math.MaxFloat32, math.SmallestNonzeroFloat32} {
		fmt.Printf("%08x %v\n", math.Float32bits(f), f)
	}
	var a float32 = math.Pi
	var b float64 = math.Pi
	fmt.Println(float64(a) == b, float32(b) == a, float64(a)-b)
	fmt.Printf("%08x %016x\n", math.Float32bits(a), math.Float64bits(float64(a)))

}

// Output:
// 402df854 2.7182817
// 40490fdb 3.1415927
// 3fcf1bbd 1.618034
// 3fb504f3 1.4142135
// 3f317218 0.6931472
// 40135d8e 2.3025851
// 7f7fffff 3.4028235e+38
// 00000001 1e-45
// false true 8.742278012618954e-08
// 40490fdb 400921fb60000000
