// Constants become C++ literals of their type: big integers, the smallest
// values, float32, and strings with octal escapes (never \x).
package main

import "fmt"

func main() {
	var i8 int8 = -128
	var u8 uint8 = 255
	var i32 int32 = -2147483648
	var u32 uint32 = 4000000000
	var i64 int64 = -9223372036854775808
	var u64 uint64 = 18446744073709551615
	var big = 1 << 62
	var neg = -3000000000
	var r = 'é'
	var f32 float32 = 0.1
	var f64 = 0.1
	var e6 = 1e6
	var whole float64 = 3
	fmt.Println(i8, u8, i32, u32, i64, u64, big, neg, r)
	fmt.Println(f32, f64, e6, whole, f32*3, -f64)
	fmt.Println("é\t\"q\" \\ \x41BC \000 ok", "")
	fmt.Println("trigraphs??= ??/ ??' ??( ??) ??! ??< ??> ??- ???")
	println(u32+1, i32-1, -neg, u64*2, big+big)
}
