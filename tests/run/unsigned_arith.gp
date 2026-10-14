// phase: 02

// Unsigned arithmetic: wrapping below zero, the biggest values, &^, ^x,
// shifts and comparisons.
package main

import "fmt"

func main() {
	var u uint = 0
	u--
	fmt.Println(u, u+1, u/2)
	var u64 uint64 = 18446744073709551615
	fmt.Println(u64, u64*u64, u64>>1, u64<<63)
	var u32 uint32 = 4000000000
	fmt.Println(u32+u32, u32*3, u32/7, u32%7)
	var m uint32 = 0xF0F0F0F0
	fmt.Println(m&^0xFF00FF00, ^m, m|0x0F, m^m, m&0x3C)
	var x uint64 = 1
	for i := range 8 {
		x = x*31 + uint64(i)
	}
	fmt.Println(x)
	a, b := uint(3), uint(5)
	fmt.Println(a-b, a-b > a, b-a)
	var uptr uintptr = 4096
	fmt.Println(uptr*2-1, uptr>>12)
	var big uint64 = 1 << 63
	fmt.Println(big, big/3, big%1000, big > 1<<62)
}

// Output:
// 18446744073709551615 0 9223372036854775807
// 18446744073709551615 1 9223372036854775807 9223372036854775808
// 3705032704 3410065408 571428571 3
// 15728880 252645135 4042322175 0 48
// 853838694149
// 18446744073709551614 true 2
// 8191 1
// 9223372036854775808 3074457345618258602 808 true
