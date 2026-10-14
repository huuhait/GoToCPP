// phase: 02

// fmt prints a float64 with the shortest digits that read back the same
// value, in exponent form when the decimal exponent is < -4 or >= 6
// (Println, %v), and %f rounds to six decimals. A grid of values around
// the edges; real Go is the reference.
package main

import "fmt"

func main() {
	x := 1.0
	for i := range 25 {
		fmt.Println(i, x, -x, x+0.5, x/3, 1/x)
		x *= 10
	}
	y := 1.0
	for i := range 12 {
		fmt.Println(-i, y, y*1.5, y/7)
		y /= 10
	}
	fmt.Println(0.1, 0.2, 0.3, 0.1+0.2, 1.0/3.0, 2.0/3.0, 100.0/3.0)
	fmt.Println(123456.0, 1234567.0, 12345678.0, 999999.0, 1000000.0, 0.0001, 0.00001, 0.000123)
	fmt.Println(1.7976931348623157e308, 4.9e-324, 2.2250738585072014e-308, 5e-324*3)
	fmt.Println(9007199254740992.0, 9007199254740993.0, 1e15+0.3, 1e16+1, 123456789012345678.0)
	fmt.Println(3.0, 3.5, -3.25, 1e100, 1.5e-100, 6.02214076e23, 6.62607015e-34)
	fmt.Printf("%v %f %v %f\n", 1e20, 1e20, 1e21, 1e21)
	fmt.Printf("%f %f %f %f %f\n", 0.1, 0.0000004, 0.0000005, 0.0000006, 123.4567895)
	fmt.Printf("%f %f %f\n", 2.5e-7, 1.5, 1.7976931348623157e308)
	fmt.Printf("%f %f %f %f\n", 0.5, 1.5, 2.5, 0.0000025)
	z := 0.0
	fmt.Println(z, -z, z*-1, 0*-1.0)
	fmt.Printf("%v %f\n", z*-1, z*-1)
}
