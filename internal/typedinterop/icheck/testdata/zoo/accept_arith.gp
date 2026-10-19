// Operators and conversions follow C++'s rules for C/C++ values: mixed arithmetic
// with Go ints, widening, narrowing to a smaller Go integer, a number to a float.
package main

import (
	"./shapes.hpp"
	"<cmath>"
	"<cstdlib>"
	"<cstring>"
	"fmt"
)

func takesFloat(f float64) float64 { return f }
func takesInt(n int) int          { return n }
func takesInt32(n int32) int32    { return n }
func takesByte(b uint8) uint8     { return b }

func main() {
	s := "abc"
	total := int(cstring.strlen("abc")) + len(s)
	big := cstring.strlen("abc") > 2
	n := int(cstring.strlen("abc"))
	f := float64(cstdlib.abs(3))
	u := uint8(cstdlib.abs(3))
	half := float64(cmath.sqrt(16) / 2)
	more := cstring.strlen("abc") + C.ulong(len(s))
	wide := cstdlib.abs(3) * 2.5
	fmt.Println(total, big, n, f, u, half, more, wide)
	fmt.Println(takesFloat(float64(cstdlib.abs(4))), takesInt(int(cstring.strlen("x"))), takesInt32(int32(cstdlib.abs(5))), takesByte(uint8(cstdlib.abs(6))))
	m := shapes.make_meters()
	d := float64(shapes.make_meters())
	fmt.Println(m.v, d, shapes.mix(1, 2), shapes.mix(2.5, 3))
	x := 7
	x = int(cstdlib.abs(-3))
	x += int(cstring.strlen("ab"))
	fmt.Println(x, int(shapes.SIZE)+x, shapes.defaults(1), shapes.defaults(1, 5))
}
