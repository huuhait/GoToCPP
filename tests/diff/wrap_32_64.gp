// phase: 02

// int32, uint32, int64, uint64, int and uint overflow wraps (-fwrapv for
// the signed ones), also around MIN and MAX.
package main

import "fmt"

func main() {
	var a int32 = 2147483647
	var b int32 = -2147483648
	fmt.Println(a+1, a*2, a*a, b-1, -b, b*-1, a+b, b*b)
	var c uint32 = 4294967295
	fmt.Println(c+1, c*c, c*2, -c, ^c, c<<31, c>>31)
	var d int64 = 9223372036854775807
	var e int64 = -9223372036854775808
	fmt.Println(d+1, d*2, d*d, e-1, -e, e*-1, d+e, e*e)
	var f uint64 = 18446744073709551615
	fmt.Println(f+1, f*f, f*2, -f, ^f, f<<63, f>>63)
	var g int = 9223372036854775807
	g++
	fmt.Println(g, g-1)
	var h uint = 0
	h -= 1
	fmt.Println(h, h/3)

	var x int32 = 1
	var y int64 = 1
	var z uint32 = 1
	for i := range 40 {
		x = x*7 + int32(i)
		y = y*31 + int64(i)
		z = z*2654435761 + uint32(i)
	}
	fmt.Println(x, y, z)

	var fact int64 = 1
	var fact32 int32 = 1
	for i := int64(1); i <= 25; i++ {
		fact *= i
		fact32 *= int32(i)
		if i >= 12 {
			fmt.Println(i, fact, fact32)
		}
	}
}
