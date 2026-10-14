// phase: 02

// Conversions between the basic number types: truncation, sign extension,
// int to float, float to int toward zero (values in range), and uint8
// from runes.
package main

import "fmt"

func main() {
	big := 1000
	fmt.Println(int8(big), uint8(big), int16(big*100), uint16(big*100))
	neg := -1
	fmt.Println(uint8(neg), uint16(neg), uint32(neg), uint64(neg), uint(neg))
	var b uint8 = 255
	fmt.Println(int8(b), int16(int8(b)), int(b), int64(int8(b)))
	var i32 int32 = -5
	fmt.Println(uint32(i32), int64(i32), uint64(uint32(i32)))
	f := 3.99
	fmt.Println(int(f), int(-f), int8(f), uint8(f), int64(f*1e9))
	fmt.Println(float64(7)/2, float32(1)/3, float64(float32(0.1)))
	var u64 uint64 = 18446744073709551615
	fmt.Println(float64(u64), float32(u64))
	var r rune = 'é'
	fmt.Println(r, uint8(r), int16(r))
	x := 65
	fmt.Println(byte(x)+1, rune(x)*2)
	h := 2.5
	fmt.Println(int(h), int(-h), int(h+0.5))
	var f32 float32 = 16777217
	fmt.Println(f32, int32(f32))
}

// Output:
// -24 232 -31072 34464
// 255 65535 4294967295 18446744073709551615 18446744073709551615
// -1 -1 255 -1
// 4294967291 -5 4294967291
// 3 -3 3 3 3990000000
// 3.5 0.33333334 0.10000000149011612
// 1.8446744073709552e+19 1.8446744e+19
// 233 233 233
// 66 130
// 2 -2 3
// 1.6777216e+07 16777216
