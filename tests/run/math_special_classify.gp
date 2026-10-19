// phase: 09
// Inf, NaN, IsInf, IsNaN, Signbit, Copysign, Frexp, Modf, Ilogb, Logb on special values.

package main

import (
	"fmt"
	"math"
)

func main() {
	vals := []float64{0, math.Copysign(0, -1), 1, -1, 0.5, -0.5, 2, -2, 10, -10, math.Pi, math.Pi / 2, -math.Pi / 2, 1e-300, 5e-324, 1e300, math.MaxFloat64, math.Inf(1), math.Inf(-1), math.NaN(), 709.7, 710, -745.2, -746, 171.5, 172, 0.9999999999999999, 1.0000000000000002}
	for _, x := range vals {
		f, e := math.Frexp(x)
		ip, fp := math.Modf(x)
		fmt.Println(x, math.IsNaN(x), math.IsInf(x, 0), math.IsInf(x, 1), math.IsInf(x, -1), math.Signbit(x), f, e, ip, fp, math.Ilogb(x), math.Logb(x))
	}
}

// Output:
// 0 false false false false false 0 0 0 0 -2147483648 -Inf
// -0 false false false false true -0 0 -0 -0 -2147483648 -Inf
// 1 false false false false false 0.5 1 1 0 0 0
// -1 false false false false true -0.5 1 -1 -0 0 0
// 0.5 false false false false false 0.5 0 0 0.5 -1 -1
// -0.5 false false false false true -0.5 0 -0 -0.5 -1 -1
// 2 false false false false false 0.5 2 2 0 1 1
// -2 false false false false true -0.5 2 -2 -0 1 1
// 10 false false false false false 0.625 4 10 0 3 3
// -10 false false false false true -0.625 4 -10 -0 3 3
// 3.141592653589793 false false false false false 0.7853981633974483 2 3 0.14159265358979312 1 1
// 1.5707963267948966 false false false false false 0.7853981633974483 1 1 0.5707963267948966 0 0
// -1.5707963267948966 false false false false true -0.7853981633974483 1 -1 -0.5707963267948966 0 0
// 1e-300 false false false false false 0.6696928794914171 -996 0 1e-300 -997 -997
// 5e-324 false false false false false 0.5 -1073 0 5e-324 -1074 -1074
// 1e+300 false false false false false 0.7466108948025751 997 1e+300 0 996 996
// 1.7976931348623157e+308 false false false false false 0.9999999999999999 1024 1.7976931348623157e+308 0 1023 1023
// +Inf false true true false false +Inf 0 +Inf NaN 2147483647 +Inf
// -Inf false true false true true -Inf 0 -Inf NaN 2147483647 +Inf
// NaN true false false false false NaN 0 NaN NaN 2147483647 NaN
// 709.7 false false false false false 0.69306640625 10 709 0.7000000000000455 9 9
// 710 false false false false false 0.693359375 10 710 0 9 9
// -745.2 false false false false true -0.727734375 10 -745 -0.20000000000004547 9 9
// -746 false false false false true -0.728515625 10 -746 -0 9 9
// 171.5 false false false false false 0.669921875 8 171 0.5 7 7
// 172 false false false false false 0.671875 8 172 0 7 7
// 0.9999999999999999 false false false false false 0.9999999999999999 0 0 0.9999999999999999 -1 -1
// 1.0000000000000002 false false false false false 0.5000000000000001 1 1 2.220446049250313e-16 0 0
