// phase: 09
// The constants of package math: values printed with %v and as bits, the integer limits as typed values.

package main

import (
	"fmt"
	"math"
)


func main() {

	fmt.Println(math.E, math.Pi, math.Phi, math.Sqrt2, math.SqrtE, math.SqrtPi, math.SqrtPhi, math.Ln2, math.Log2E, math.Ln10, math.Log10E)
	fmt.Printf("%016x %016x %016x %016x\n", math.Float64bits(math.E), math.Float64bits(math.Pi), math.Float64bits(math.Phi), math.Float64bits(math.Sqrt2))
	fmt.Printf("%016x %016x %016x %016x\n", math.Float64bits(math.SqrtE), math.Float64bits(math.SqrtPi), math.Float64bits(math.SqrtPhi), math.Float64bits(math.Ln2))
	fmt.Printf("%016x %016x %016x\n", math.Float64bits(math.Log2E), math.Float64bits(math.Ln10), math.Float64bits(math.Log10E))
	fmt.Println(math.MaxFloat32, math.SmallestNonzeroFloat32, math.MaxFloat64, math.SmallestNonzeroFloat64)
	fmt.Printf("%016x %016x %08x %08x\n", math.Float64bits(math.MaxFloat64), math.Float64bits(math.SmallestNonzeroFloat64), math.Float32bits(math.MaxFloat32), math.Float32bits(math.SmallestNonzeroFloat32))
	fmt.Println(math.MaxInt8, math.MinInt8, math.MaxInt16, math.MinInt16, math.MaxInt32, math.MinInt32, math.MaxInt64, math.MinInt64)
	fmt.Println(math.MaxUint8, math.MaxUint16, math.MaxUint32, uint64(math.MaxUint64))
	fmt.Println(math.MaxInt, math.MinInt, uint(math.MaxUint))
	var i8 int8 = math.MaxInt8
	var i64 int64 = math.MinInt64
	var u64 uint64 = math.MaxUint64
	fmt.Println(i8, i64, u64, u64+1, i64-1, -i64)

}

// Output:
// 2.718281828459045 3.141592653589793 1.618033988749895 1.4142135623730951 1.6487212707001282 1.772453850905516 1.272019649514069 0.6931471805599453 1.4426950408889634 2.302585092994046 0.4342944819032518
// 4005bf0a8b145769 400921fb54442d18 3ff9e3779b97f4a8 3ff6a09e667f3bcd
// 3ffa61298e1e069c 3ffc5bf891b4ef6b 3ff45a3146a88456 3fe62e42fefa39ef
// 3ff71547652b82fe 40026bb1bbb55516 3fdbcb7b1526e50e
// 3.4028234663852886e+38 1.401298464324817e-45 1.7976931348623157e+308 5e-324
// 7fefffffffffffff 0000000000000001 7f7fffff 00000001
// 127 -128 32767 -32768 2147483647 -2147483648 9223372036854775807 -9223372036854775808
// 255 65535 4294967295 18446744073709551615
// 9223372036854775807 -9223372036854775808 18446744073709551615
// 127 -9223372036854775808 18446744073709551615 0 9223372036854775807 -9223372036854775808
