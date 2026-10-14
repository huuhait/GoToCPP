// Division and modulo: gp::div and gp::rem (MIN / -1 is MIN, MIN % -1 is 0,
// truncation toward zero, the sign of the dividend), and the plain C++
// operator for a constant divisor other than 0 and -1.
package main

import "fmt"

func main() {
	var a, b int = 7, -2
	fmt.Println(a/b, a%b, -a/b, -a%b, a/-b, a%-b, -a/-b, -a%-b)
	fmt.Println(a/2, a%2, -a/2, -a%2, a/-3, a%-3, b/2, b%2)
	m1 := -1
	var i64 int64 = -9223372036854775808
	fmt.Println(i64/int64(m1), i64%int64(m1), i64/-1, i64%-1, i64/2, i64%2, i64/-2)
	var i32 int32 = -2147483648
	fmt.Println(i32/int32(m1), i32%int32(m1), i32/-1, i32%-1, i32/7, i32%7)
	var i16 int16 = -32768
	var i8 int8 = -128
	fmt.Println(i16/int16(m1), i16%int16(m1), i16/-1, i8/-1, i8%-1, i8/int8(m1), i8%int8(m1))
	var u uint = 18446744073709551615
	var u32 uint32 = 4294967295
	var u8 uint8 = 250
	fmt.Println(u/3, u%10, u/u, u32/7, u32%7, u8/3, u8%7, u8/u8)
	var two uint64 = 2
	fmt.Println(uint64(u)/two, uint64(u)%two, 100/a, 100%a, -100/a)
	var f, g float64 = 7, -2
	fmt.Println(f/g, f/2, -f/4, g/f)
	var f32 float32 = 1
	fmt.Println(f32/3, f32/3*3)
}
