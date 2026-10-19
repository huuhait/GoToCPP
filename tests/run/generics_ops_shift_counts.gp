// phase: 07

// Shifts where the count has its own type parameter: signed, unsigned and negative counts.
package main

import "fmt"

type Int interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

type Count interface {
	~int | ~int8 | ~uint | ~uint8 | ~uint64
}

func Shl[T Int, N Count](a T, n N) T { return a << n }
func Shr[T Int, N Count](a T, n N) T { return a >> n }
func ShlEq[T Int, N Count](a T, n N) T {
	a <<= n
	a >>= 1
	return a
}

func try[T Int, N Count](a T, n N) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("%T %v<<%v panic: %v\n", a, a, n, r)
		}
	}()
	fmt.Printf("%T %v %d %d %d\n", a, a, n, Shl(a, n), Shr(a, n))
}

func main() {
	try[int8, uint](1, 7)
	try[int8, uint](-1, 9)
	try[int8, uint8](-128, 255)
	try[uint8, int](255, 4)
	try[uint8, int](255, 8)
	try[uint8, int](255, -1)
	try[int16, int8](-1, -5)
	try[int16, int8](1, 15)
	try[uint16, uint64](1, 1<<63)
	try[int32, uint64](-2, 33)
	try[int64, int](1, 63)
	try[int64, int](-1, 64)
	try[uint64, uint8](1<<63, 63)
	fmt.Println(ShlEq[uint8, uint](200, 1), ShlEq[int8, int](100, 2), ShlEq[int32, uint8](1, 31))
}

// Output:
// int8 1 7 -128 0
// int8 -1 9 0 -1
// int8 -128 255 0 -1
// uint8 255 4 240 15
// uint8 255 8 0 0
// uint8 255<<-1 panic: runtime error: negative shift amount
// int16 -1<<-5 panic: runtime error: negative shift amount
// int16 1 15 -32768 0
// uint16 1 9223372036854775808 0 0
// int32 -2 33 0 -1
// int64 1 63 -9223372036854775808 0
// int64 -1 64 0 -1
// uint64 9223372036854775808 63 0 1
// 72 -56 -1073741824
