// The small-integer rule: C++ computes int8, int16, uint8 and uint16 in int,
// so every operator on them is cast back to the Go type, and the results wrap
// like Go's. Comparisons need no cast.
package main

import "fmt"

func digit(s string, i int) bool {
	return s[i]-'0' <= 9
}

func add8(a, b uint8) uint8 {
	return a + b
}

func main() {
	var a, b uint8 = 200, 100
	fmt.Println(a+b, a+b > 250, a-b-b-b, a*b, -a, ^a, +a)
	fmt.Println(add8(a, b), add8(a, b)/2, (a+b)/2, a/b, a%b, a/3, a%7)
	var i8, m1 int8 = -128, -1
	fmt.Println(i8*m1, -i8, i8-1, i8+i8, ^i8, i8/m1, i8%m1, i8*-1, i8/2)
	var x16 int16 = 32767
	var u16 uint16 = 65535
	fmt.Println(x16+1, x16*x16, -x16-2, u16+1, u16*u16, u16<<1, u16<<15, u16>>15, u16<<16)
	fmt.Println(u16&0xff00, u16|1, u16^0x0f0f, u16&^0x00ff, ^u16)
	println(a+b, i8*m1, x16+1, u16+1)
	fmt.Println(digit("7x", 0), digit("7x", 1), digit("a/", 1))
	// Comparisons of small integers compare the wrapped values.
	var c, d uint8 = 255, 0
	fmt.Println(c+1 == d, c+1 < d, c > d, i8 < m1, i8 == -128, u16+1 == 0)
	// Small integers mixed with constants and in nested expressions.
	var e int8 = 100
	fmt.Println(e+e+e, (e+e)/2, e*2/2, e<<1>>1, -(-e - e))
	var r rune = 'a'
	var by byte = 'z'
	fmt.Println(r+1, by+10, by-'a'+'A')
}
