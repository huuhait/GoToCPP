// Float output: fmt's %v gives the shortest digits that read back as the same
// float, in exponent form when the exponent is < -4 or >= 6 (123456, 1e+06,
// 1.234567e+06, 0.0001, 1e-05); the builtin println uses Go's runtime format.
// Special values: +Inf, -Inf, NaN, and -0 for a negative zero (a constant
// -0.0 is 0). Constants are exact (0.1+0.2 == 0.3), float64 values are not.
package main

import "fmt"

func main() {
	vals := 0.0
	a, b, c, d, e := 123456.0+vals, 1000000.0+vals, 1234567.0+vals, 0.0001+vals, 0.00001+vals
	fmt.Println(a, b, c, d, e)
	fmt.Println(100000.0+vals, 999999.0+vals, 1e20+vals, 1e21+vals, 123456789.0+vals, 0.000123+vals)
	zero := 0.0
	inf, ninf, nan, negz := 1/zero, -1/zero, zero/zero, -zero
	fmt.Println(inf, ninf, nan, negz, zero, -0.0, 1/negz)
	println(inf, ninf, nan, negz, zero, 1.5, 3.0, -0.25)
	fmt.Printf("%v %f %v %f %v\n", inf, ninf, nan, negz, 2.5)
	x, y := 0.1, 0.2
	fmt.Println(x+y, x*y, x-y, y/x, 0.1+0.2, x+y == 0.3, 0.1+0.2 == 0.3)
	var f32 float32 = 0.1
	var g32 float32 = 16777216
	fmt.Println(f32, f32+f32+f32, f32*f32, g32+1, g32+2, float32(1)/3, f32/3)
	println(f32, g32+1)
	var big float32 = 3.4028235e38
	fmt.Println(big, big*2, -big*2, big*2-big*2, float32(1e-45), float32(1e-45)/2)
	fmt.Println(5e-324+zero, (5e-324+zero)/2, 1.7976931348623157e308+zero, (1.7976931348623157e308+zero)*2)
	fmt.Println(float64(f32), float32(x), float64(float32(0.1)), float32(1e40*(x+1)))
}
