// phase: 02

// float32 values print with the shortest digits for float32 (not for the
// float64 with the same value), and float32 arithmetic rounds to float32
// after each operation.
package main

import "fmt"

func main() {
	var f float32 = 0.1
	fmt.Println(f, f*3, f+0.2, float64(f), f*f)
	x := float32(1)
	for i := range 15 {
		fmt.Println(i, x, x/3, -x/7, 1/x)
		x *= 10
	}
	fmt.Println(float32(16777216), float32(16777217), float32(16777218), float32(3.4028235e38))
	fmt.Println(float32(1.4e-45), float32(1.17549435e-38), float32(123456.7), float32(1234567.8))
	var acc float32
	for range 1000 {
		acc += 0.001
	}
	fmt.Println(acc, acc == 1)
	var a, b float32 = 1e8, 1
	fmt.Println(a+b, a+b == a, a*b, (a+b)-a)
	fmt.Printf("%v %f %v %f\n", f, f, float32(2.5e-7), float32(1e20))
	var z float32
	fmt.Println(z, -z, z*-1)
}
