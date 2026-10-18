// phase: 07

// unsafe.Sizeof and Alignof of a type parameter's value are constants in each instance.
package main

import (
	"fmt"
	"unsafe"
)

type Pair struct {
	a int8
	b int64
}

type Small struct{ a, b int16 }

func Size[T any](v T) uintptr { return unsafe.Sizeof(v) }

func Align[T any]() uintptr {
	var z T
	return unsafe.Alignof(z)
}

func Bits[T ~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64](x T) int {
	return int(unsafe.Sizeof(x)) * 8
}

func Total[T any](xs []T) uintptr { return uintptr(len(xs)) * unsafe.Sizeof(xs[0]) }

func main() {
	fmt.Println(Size(int8(1)), Size(int16(1)), Size(int32(1)), Size(int64(1)), Size(1), Size(1.5), Size(float32(1)), Size(true))
	fmt.Println(Size(Small{}), Size(Pair{}), Size([3]int16{}), Size([2]Small{}), Size(struct{}{}) >= 0)
	fmt.Println(Align[int8](), Align[int64](), Align[Small](), Align[Pair](), Align[[4]int16]())
	fmt.Println(Bits(int8(0)), Bits(uint16(0)), Bits(int32(0)), Bits(uint64(0)))
	fmt.Println(Total([]int32{1, 2, 3}), Total([]Small{{}, {}}), Total([]Pair{{}}))
}

// Output:
// 1 2 4 8 8 8 4 1
// 4 16 6 8 true
// 1 8 2 8 2
// 8 16 32 64
// 12 8 16
