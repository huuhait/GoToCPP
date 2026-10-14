// phase: 02

// Signed division and modulo truncate toward zero, the remainder has the
// sign of the dividend, and MIN / -1 is MIN and MIN % -1 is 0 for every
// signed size (C++ makes these undefined or traps).
package main

import "fmt"

func i8s(i int) int8 {
	switch i {
	case 0:
		return -128
	case 1:
		return -7
	case 2:
		return -1
	case 3:
		return 1
	case 4:
		return 3
	}
	return 127
}

func main() {
	for i := range 6 {
		for j := range 6 {
			a, b := i8s(i), i8s(j)
			fmt.Println(a, b, a/b, a%b)
			a16, b16 := int16(a)*256, int16(b)
			fmt.Println(a16, b16, a16/b16, a16%b16)
			a32, b32 := int32(a)*16777216, int32(b)
			fmt.Println(a32, b32, a32/b32, a32%b32)
			a64, b64 := int64(a)*72057594037927936, int64(b)
			fmt.Println(a64, b64, a64/b64, a64%b64)
			ai, bi := int(a64), int(b)
			fmt.Println(ai, bi, ai/bi, ai%bi)
		}
	}
	var m8 int8 = -128
	var m16 int16 = -32768
	var m32 int32 = -2147483648
	var m64 int64 = -9223372036854775808
	var mi int = -9223372036854775808
	one := -1
	fmt.Println(m8/int8(one), m8%int8(one), m16/int16(one), m16%int16(one))
	fmt.Println(m32/int32(one), m32%int32(one), m64/int64(one), m64%int64(one), mi/one, mi%one)
	m8 /= int8(one)
	m16 %= int16(one)
	m32 /= int32(one)
	m64 /= int64(one)
	fmt.Println(m8, m16, m32, m64)
	fmt.Println(-7/2, -7%2, 7/-2, 7%-2, -7/-2, -7%-2)
	x := -7
	fmt.Println(x/2, x%2, x/-2, x%-2, x/4, x%4, x/8, x%8)
	fmt.Println(x>>1, x/2, -x>>1)
}
