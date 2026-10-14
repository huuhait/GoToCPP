// phase: 02

// Bit tricks: counting set bits two ways, the lowest set bit, powers of
// two, reversing the bits of a byte and a uint32, and parity.
package main

import "fmt"

func popcount(x uint64) int {
	n := 0
	for x != 0 {
		x &= x - 1
		n++
	}
	return n
}

func popcountShift(x uint32) int {
	n := 0
	for i := range 32 {
		n += int(x >> i & 1)
	}
	return n
}

func lowest(x int64) int64 {
	return x & -x
}

func isPow2(x uint) bool {
	return x != 0 && x&(x-1) == 0
}

func reverse8(b uint8) uint8 {
	var r uint8
	for range 8 {
		r = r<<1 | b&1
		b >>= 1
	}
	return r
}

func reverse32(x uint32) uint32 {
	x = x>>1&0x55555555 | x&0x55555555<<1
	x = x>>2&0x33333333 | x&0x33333333<<2
	x = x>>4&0x0F0F0F0F | x&0x0F0F0F0F<<4
	x = x>>8&0x00FF00FF | x&0x00FF00FF<<8
	return x>>16 | x<<16
}

func parity(x uint64) uint64 {
	x ^= x >> 32
	x ^= x >> 16
	x ^= x >> 8
	x ^= x >> 4
	x ^= x >> 2
	x ^= x >> 1
	return x & 1
}

func main() {
	fmt.Println(popcount(0), popcount(255), popcount(18446744073709551615), popcount(0x8000000000000001))
	fmt.Println(popcountShift(0), popcountShift(0xFFFFFFFF), popcountShift(0xA5A5A5A5))
	fmt.Println(lowest(12), lowest(-8), lowest(0), lowest(-9223372036854775808))
	for i := uint(0); i <= 16; i++ {
		if isPow2(i) {
			fmt.Print(i, " ")
		}
	}
	fmt.Println("|")
	fmt.Println(reverse8(1), reverse8(0x0F), reverse8(0xA0), reverse8(255))
	fmt.Println(reverse32(1), reverse32(0x12345678), reverse32(0xFFFF0000))
	fmt.Println(parity(7), parity(255), parity(1<<40|1))
	var flags uint8 = 0b1010_1100
	flags |= 1 << 0
	flags &^= 1 << 3
	flags ^= 0xF0
	fmt.Println(flags, flags>>4, flags&0x0F)
}

// Output:
// 0 8 64 2
// 0 32 16
// 4 8 0 -9223372036854775808
// 1 2 4 8 16 |
// 128 240 5 255
// 2147483648 510274632 65535
// 1 0 0
// 85 5 5
