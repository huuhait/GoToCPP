// phase: 02

// Unsigned division and modulo for every unsigned size, with big values
// and powers of two, and constant divisors next to variable ones.
package main

import "fmt"

func main() {
	var u8 uint8 = 255
	var u16 uint16 = 65535
	var u32 uint32 = 4294967295
	var u64 uint64 = 18446744073709551615
	var u uint = 18446744073709551615
	for d := uint64(1); d < 20; d += 3 {
		fmt.Println(d, u8/uint8(d), u8%uint8(d), u16/uint16(d), u16%uint16(d),
			u32/uint32(d), u32%uint32(d), u64/d, u64%d, u/uint(d), u%uint(d))
	}
	fmt.Println(u8/2, u8%2, u16/256, u16%256, u32/65536, u64/4294967296, u64%4294967296)
	fmt.Println(u64/u64, u64%u64, u64/(u64-1), u64%(u64-1))
	big := uint64(1) << 63
	fmt.Println(big/3, big%3, big/7, big%7, (big+1)/2)
	var b uint8 = 200
	var c uint8 = 7
	fmt.Println(b/c, b%c, (b+c)/c, (b*c)%c, b*c/c)
}
