// phase: 04

// Conversions of C/C++ floats out of range (1e300, NaN) give Go's results (amd64), through
// gp::cgo::as and gp::f2i: a plain static_cast would be undefined behavior (UBSan in -asan).
// The output is Go 1.25's for the same program with Go's types (math.Pow, int32(x), ...).
package main

import (
	"fmt"

	"<cmath>"
)

func main() {
	big := cmath.pow(10.0, 300.0)
	nan := cmath.nan("")
	fmt.Println(int(big), int(nan), int32(big), uint8(big), uint64(big))
	fmt.Println(int(C.int(big)), int(C.ulonglong(nan)), float64(C.float(big)), float32(big))
	x, y := 1e300, -2.9
	fmt.Println(int(C.int(x)), float64(C.float(x)), int(C.longlong(y)))
	fmt.Println(int(cmath.pow(10, 30)), int64(cmath.pow(2, 62)))
}

// Output:
// -9223372036854775808 -9223372036854775808 -2147483648 0 9223372036854775808
// -2147483648 -9223372036854775808 +Inf +Inf
// -2147483648 +Inf -2
// -9223372036854775808 4611686018427387904
