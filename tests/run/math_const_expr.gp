// phase: 09
// Constant expressions are exact (arbitrary precision) and rounded once to float64 or float32, as in Go.

package main

import (
	"fmt"
	"math"
)


func main() {

	const big = 1 << 100
	const third = 1.0 / 3
	const huge = math.MaxFloat64 * 2 / 4
	const tiny = math.SmallestNonzeroFloat64 * 1e300 * 1e10 / 1e310
	fmt.Println(float64(big)/(1<<98), third, huge, tiny)
	fmt.Println(math.Pi*1e300/1e300, math.Pi*math.Pi/math.Pi == math.Pi, math.E*math.E)
	fmt.Printf("%016x %016x %016x\n", math.Float64bits(third), math.Float64bits(huge), math.Float64bits(tiny))
	const x = 0.1 + 0.2
	var y = 0.1
	y += 0.2
	fmt.Println(x, y, x == 0.3, y == 0.3)
	fmt.Printf("%016x %016x\n", math.Float64bits(x), math.Float64bits(y))
	const f32 float32 = 0.1
	fmt.Println(f32, float64(f32), f32*3, float64(f32*3))
	const c = math.MaxInt64 / 3 * 2.5
	fmt.Println(c, math.MaxUint64/7, uint64(math.MaxUint64/7))
	const e = 1e23
	const f = 8.41e21
	fmt.Println(e, f, float32(e), float32(f))
	fmt.Printf("%016x %016x %08x\n", math.Float64bits(e), math.Float64bits(f), math.Float32bits(float32(e)))
	var one = 1.0
	fmt.Println(one/3 == third, (one/3)*3, math.Pi*one == math.Pi)

}

// Output:
// 4 0.3333333333333333 8.988465674311579e+307 5e-324
// 3.141592653589793 true 7.38905609893065
// 3fd5555555555555 7fdfffffffffffff 0000000000000001
// 0.3 0.30000000000000004 true false
// 3fd3333333333333 3fd3333333333334
// 0.1 0.10000000149011612 0.3 0.30000001192092896
// 7.686143364045647e+18 2635249153387078802 2635249153387078802
// 1e+23 8.41e+21 1e+23 8.41e+21
// 44b52d02c7e14af6 447c7e83209e90b2 65a96816
// true 1 true
