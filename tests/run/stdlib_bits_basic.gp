// phase: 09

// math/bits (ported): counting, leading and trailing zeros, rotation, reversal, and the carry and
// double-width arithmetic that the standard library builds on, including the overflow panic of Div64.
package main

import (
	"fmt"
	"math/bits"
)

func main() {
	for _, x := range []uint64{0, 1, 2, 0x80, 0xff, 0x1234567890abcdef, 1 << 63, ^uint64(0)} {
		fmt.Println(x, bits.OnesCount64(x), bits.LeadingZeros64(x), bits.TrailingZeros64(x), bits.Len64(x),
			bits.Reverse64(x), bits.ReverseBytes64(x), bits.RotateLeft64(x, 13), bits.RotateLeft64(x, -7))
	}
	for _, x := range []uint32{0, 1, 0xf0f0f0f0, 0xdeadbeef, ^uint32(0)} {
		fmt.Println(x, bits.OnesCount32(x), bits.LeadingZeros32(x), bits.TrailingZeros32(x), bits.Len32(x),
			bits.Reverse32(x), bits.ReverseBytes32(x), bits.RotateLeft32(x, 5))
	}
	for _, x := range []uint8{0, 1, 0x81, 0xff} {
		fmt.Println(x, bits.OnesCount8(x), bits.LeadingZeros8(x), bits.TrailingZeros8(x), bits.Len8(x), bits.Reverse8(x), bits.RotateLeft8(x, 3))
	}
	fmt.Println(bits.UintSize, bits.OnesCount(255), bits.Len(1023), bits.LeadingZeros(1), bits.TrailingZeros(8), bits.Reverse16(0x00ff), bits.ReverseBytes16(0x1234))

	sum, carry := bits.Add64(^uint64(0), 1, 0)
	fmt.Println(sum, carry)
	sum, carry = bits.Add64(^uint64(0), ^uint64(0), 1)
	fmt.Println(sum, carry)
	diff, borrow := bits.Sub64(0, 1, 0)
	fmt.Println(diff, borrow)
	hi, lo := bits.Mul64(0xffffffffffffffff, 0xffffffffffffffff)
	fmt.Println(hi, lo)
	hi, lo = bits.Mul64(0x123456789, 0x987654321)
	fmt.Println(hi, lo)
	q, r := bits.Div64(1, 0, 3)
	fmt.Println(q, r)
	q, r = bits.Div64(0, 100, 7)
	fmt.Println(q, r)
	fmt.Println(bits.Rem64(5, 7, 11))
	s32, c32 := bits.Add32(0xffffffff, 2, 0)
	hi32, lo32 := bits.Mul32(0xffffffff, 0xffffffff)
	fmt.Println(s32, c32, hi32, lo32)

	defer func() {
		fmt.Println("recovered:", recover())
	}()
	bits.Div64(5, 0, 5)
}

// Output:
// 0 0 64 64 0 0 0 0 0
// 1 1 63 0 1 9223372036854775808 72057594037927936 8192 144115188075855872
// 2 1 62 1 2 4611686018427387904 144115188075855872 16384 288230376151711744
// 128 1 56 7 8 72057594037927936 9223372036854775808 1048576 1
// 255 8 56 0 8 18374686479671623680 18374686479671623680 2088960 18302628885633695745
// 1311768467294899695 32 3 0 61 17848843983341104200 17279655982273016850 10002233180859261510 16007034067570743195
// 9223372036854775808 1 0 63 64 1 128 4096 72057594037927936
// 18446744073709551615 64 0 0 64 18446744073709551615 18446744073709551615 18446744073709551615 18446744073709551615
// 0 0 32 32 0 0 0 0
// 1 1 31 0 1 2147483648 16777216 32
// 4042322160 16 0 4 32 252645135 4042322160 505290270
// 3735928559 24 0 0 32 4152210811 4022250974 3585596923
// 4294967295 32 0 0 32 4294967295 4294967295 4294967295
// 0 0 8 8 0 0 0
// 1 1 7 0 1 128 8
// 129 2 0 0 8 129 12
// 255 8 0 0 8 255 255
// 64 8 10 63 3 65280 13330
// 0 1
// 18446744073709551615 1
// 18446744073709551615 1
// 18446744073709551614 1
// 10 15527694826028413865
// 6148914691236517205 1
// 14 2
// 10
// 1 1 4294967294 1
// recovered: runtime error: integer overflow
