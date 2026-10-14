// The bit operators: & | ^ the same in C++, &^ as a & ~b, unary ^ as ~, on
// signed and unsigned types, and Go's precedence (& and &^ bind like *, | and ^
// like +) written with C++'s.
package main

import "fmt"

func main() {
	var a, b int = 12, 10
	fmt.Println(a&b, a|b, a^b, a&^b, ^a, ^-a, -^a, ^0+a)
	var u, v uint = 12, 10
	fmt.Println(u&v, u|v, u^v, u&^v, ^u, ^v&^u, u&^0, u&^1)
	var i8 int8 = -128
	var u8 uint8 = 0xF0
	fmt.Println(^i8, i8&^1, i8|1, i8^-1, ^u8, u8&^0x30, u8^0xFF, u8|0x0F, u8&0x3C)
	var i32 int32 = -1
	var u32 uint32 = 0xFFFF0000
	fmt.Println(^i32, ^u32, u32&^0xFF000000, i32&^0x7FFFFFFF, u32^0x0000FFFF)
	var i64 int64 = -9223372036854775808
	var u64 uint64 = 0x8000000000000001
	fmt.Println(^i64, i64&^-1, ^u64, u64&^1, u64&^(1<<63), u64|1<<62)
	// Precedence: Go's & binds tighter than +, C++'s doesn't.
	fmt.Println(a&b+1, a+b&1, a|b+1, a+b|1, a^b*2, a*b^2, a&^b+1, a+b&^1, a&b == 8, a|b != 0)
	fmt.Println(a&b|a^b, (a|b)&(a^b), a<<1&b, a&b<<1, -a&b, ^a&b, a&-b)
	var x uint16 = 0x1234
	fmt.Println(x&0xFF<<8|x>>8, ^x, x&^x, -x, x^x^x)
}
