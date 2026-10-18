// phase: 07

// Conversions between type parameters and to concrete types: truncation, sign, float to int.
package main

import "fmt"

type Int interface {
	~int8 | ~uint8 | ~int16 | ~uint16 | ~int32 | ~int64 | ~uint64 | ~int
}

type Num interface{ Int | ~float32 | ~float64 }

type Level uint8

func Conv[T, U Num](x T) U { return U(x) }

func ToF[T Num](x T) float64 { return float64(x) }

func ToInt[T Num](x T) int { return int(x) }

func FromInt[T Num](x int) T { return T(x) }

func Widen[T Int](x T) int64 { return int64(x) }

func ToString[T ~int32 | ~uint8](x T) string { return string(rune(x)) }

func Same[T Num](x T) T { return T(x) }

func main() {
	fmt.Println(Conv[int, int8](300), Conv[int, uint8](-1), Conv[int8, int](-5), Conv[uint8, int8](200), Conv[int64, uint64](-1))
	fmt.Println(Conv[float64, int](3.99), Conv[float64, int](-3.99), Conv[float64, int8](100.9), Conv[float32, uint8](200.5))
	fmt.Println(Conv[int, float32](16777217), Conv[uint64, float64](1<<63), Conv[int8, float64](-7))
	fmt.Println(ToF(int8(-3)), ToF(uint64(1<<63)), ToF(float32(0.1)), ToF(Level(200)))
	fmt.Println(ToInt(2.9), ToInt(Level(255)), ToInt(int8(-1)), ToInt(uint64(1<<40)))
	fmt.Println(FromInt[int8](200), FromInt[uint8](257), FromInt[float64](3), FromInt[Level](256+7))
	fmt.Println(Widen(int8(-128)), Widen(uint8(255)), Widen(uint64(1<<63)), Widen(Level(7)))
	fmt.Println(ToString(int32(65)), ToString(uint8(0x41)), ToString(int32(0x4e16)))
	fmt.Println(Same(5), Same(uint8(250)), Same(2.5))
	fmt.Printf("%T %T %T\n", Conv[int, Level](1), Same(Level(1)), Conv[Level, float32](1))
}

// Output:
// 44 255 -5 -56 18446744073709551615
// 3 -3 100 200
// 1.6777216e+07 9.223372036854776e+18 -7
// -3 9.223372036854776e+18 0.10000000149011612 200
// 2 255 -1 1099511627776
// -56 1 3 7
// -128 255 -9223372036854775808 7
// A A 世
// 5 250 2.5
// main.Level main.Level float32
