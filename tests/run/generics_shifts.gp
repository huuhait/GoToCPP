// phase: 07

// Shifts in generic code: unsigned and signed counts, counts larger than the width, and constants.
package main

import "fmt"

type Int interface {
	~int8 | ~uint8 | ~int16 | ~uint32 | ~int64 | ~uint64 | ~int
}

func Shl[T Int](a T, n uint) T   { return a << n }
func Shr[T Int](a T, n uint) T   { return a >> n }
func ShlI[T Int](a T, n int) T   { return a << n }
func One[T Int](n uint) T        { return 1 << n }
func Bit[T Int](a T, i uint) bool { return a>>i&1 == 1 }

func Popcount[T ~uint8 | ~uint64 | ~uint32](x T) int {
	n := 0
	for x != 0 {
		if x&1 != 0 {
			n++
		}
		x >>= 1
	}
	return n
}

func main() {
	fmt.Println(Shl[int8](1, 7), Shl[uint8](1, 7), Shl[uint8](255, 1), Shl[int16](1, 15), Shl[int8](1, 8), Shl[uint32](1, 40))
	fmt.Println(Shr[int8](-128, 3), Shr[uint8](128, 3), Shr[int64](-1, 70), Shr[uint64](1<<63, 63), Shr[int](-5, 1))
	fmt.Println(ShlI[int8](3, 2), ShlI[uint64](1, 63), ShlI[int](1, 0))
	fmt.Println(One[int8](7), One[uint8](7), One[int](62), One[uint64](63), One[int8](9))
	fmt.Println(Bit[int8](5, 0), Bit[int8](5, 1), Bit[uint64](1<<40, 40), Bit[int](-1, 63))
	fmt.Println(Popcount[uint8](255), Popcount[uint8](255), Popcount[uint32](1<<31|1), Popcount[uint64](1<<63|1))
	defer func() { fmt.Println("recovered:", recover()) }()
	fmt.Println(ShlI[int](1, -1))
}

// Output:
// -128 128 254 -32768 0 0
// -16 16 -1 1 -3
// 12 9223372036854775808 1
// -128 128 4611686018427387904 9223372036854775808 0
// true false true true
// 8 8 2 2
// recovered: runtime error: negative shift amount
