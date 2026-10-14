// Shifts: gp::shl and gp::shr for a variable count or a constant one of the
// width or more (Go gives 0, or -1 for >> of a negative signed value), the
// plain C++ operator for a constant count below the width.
package main

import "fmt"

func main() {
	var x int64 = -5
	var n uint = 3
	fmt.Println(x<<n, x>>n, x<<3, x>>3, x>>63, x>>64, x>>100, x<<63, x<<64)
	var y uint64 = 0xF0000000000000F0
	fmt.Println(y<<n, y>>n, y<<4, y>>4, y>>63, y>>64, y<<64, y<<200)
	big := uint(64)
	huge := uint(1 << 40)
	fmt.Println(x<<big, x>>big, y<<big, y>>big, x>>huge, y>>huge)
	var u16 uint16 = 0xABCD
	var i16 int16 = -21555
	var c8 uint8 = 7
	fmt.Println(u16<<c8, u16>>c8, u16<<8, u16>>8, u16<<15, u16<<16, u16>>16, u16<<(c8+9))
	fmt.Println(i16<<c8, i16>>c8, i16<<1, i16>>1, i16>>15, i16>>16, i16<<16)
	var u8 uint8 = 0x81
	var i8 int8 = -127
	fmt.Println(u8<<1, u8>>1, u8<<7, u8<<8, i8<<1, i8>>1, i8<<7, i8>>7, i8>>8)
	var i32 int32 = -1
	var u32 uint32 = 1
	fmt.Println(i32<<31, i32>>31, i32<<32, u32<<31, u32<<32, u32<<n, i32>>n)
	// Signed counts and the untyped constant on the left of a variable shift.
	var s int8 = 2
	one := 1 << n
	var w uint32 = 1 << n
	fmt.Println(x<<s, y>>s, one, w, 1<<s, int8(1)<<6)
	fmt.Println(x<<n+1, x<<(n+1), x+1<<n, x+1>>n)
}
