// phase: 09
// Integer limits from math in arithmetic: wrapping at MaxInt64 and MaxUint64, conversions of MinInt64, shifts, and constant expressions with them.

package main

import (
	"fmt"
	"math"
)


func main() {

	var a int64 = math.MaxInt64
	var b int64 = math.MinInt64
	var c uint64 = math.MaxUint64
	var d int32 = math.MaxInt32
	var e int8 = math.MinInt8
	fmt.Println(a+1, b-1, -b, c+1, c*2, d+1, e-1, -e, a*2, b/-1, uint64(b), int64(c), uint8(c), int8(a))
	fmt.Println(math.MaxInt64/2+1, uint64(math.MaxUint64)>>1 == math.MaxInt64, math.MinInt64 == -math.MaxInt64-1, float64(math.MaxInt64), float64(math.MinInt64), float64(uint64(math.MaxUint64)))
	fv := 0.5
	u32 := uint32(math.MaxUint32)
	fmt.Println(float32(math.MaxInt32), int64(float64(math.MaxInt32)+fv), u32+1)

}

// Output:
// -9223372036854775808 9223372036854775807 -9223372036854775808 0 18446744073709551614 -2147483648 127 -128 -2 -9223372036854775808 9223372036854775808 -1 255 -1
// 4611686018427387904 true true 9.223372036854776e+18 -9.223372036854776e+18 1.8446744073709552e+19
// 2.1474836e+09 2147483647 0
