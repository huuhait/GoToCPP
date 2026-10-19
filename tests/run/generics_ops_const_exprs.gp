// phase: 07

// Constants used with type parameters: conversions, typed expressions, overflow of values at run time.
package main

import "fmt"

type Int8s interface{ ~int8 | ~uint8 | ~int16 | ~int64 }
type Num interface {
	~int8 | ~uint8 | ~int | ~int64 | ~uint64 | ~float32 | ~float64
}

const Big = 1 << 40
const Third = 1.0 / 3

func Const7[T Num]() T            { return 7 }
func Twice[T Num](x T) T          { return x * 2 }
func Half[T ~float32 | ~float64](x T) T { return x * 0.5 }
func Fill[T Num](n int) []T {
	s := make([]T, n)
	for i := range s {
		s[i] = T(i*3 + 1)
	}
	return s
}
func Offset[T Num](x T) T         { return x + 100 }
func Shifted[T Int8s]() T         { return T(1)<<2 | 3 }
func Max127[T Int8s]() T          { return 127 }
func MaskLow[T ~uint8 | ~uint64](x T) T { return x & 0x0f }
func Pct[T ~float64 | ~float32](x T) T { return x * 100 / 100 }
func Cond[T Num](x T) string {
	switch {
	case x > 100:
		return "big"
	case x == 7:
		return "seven"
	}
	return "small"
}
func Third3[T ~float32 | ~float64]() T { return Third }

func main() {
	fmt.Println(Const7[int8](), Const7[uint8](), Const7[int](), Const7[float32](), Const7[float64](), Const7[uint64]())
	fmt.Println(Twice(int8(100)), Twice(uint8(200)), Twice(2.25), Twice(float32(1.1)), Half(5.0), Half(float32(5)))
	fmt.Println(Fill[int8](5), Fill[uint8](4), Fill[float32](3), len(Fill[uint64](0)))
	fmt.Println(Offset(int8(50)), Offset(uint8(200)), Offset(0.5), Offset(uint64(1)<<63))
	fmt.Println(Shifted[int8](), Shifted[uint8](), Shifted[int16](), Max127[int8](), Max127[int64](), Max127[uint8]())
	fmt.Println(MaskLow(uint8(0xAB)), MaskLow(uint64(0xFFFF)), Pct(3.5), Pct(float32(2.5)))
	fmt.Println(Cond(7), Cond(101), Cond(int8(-5)), Cond(7.0), Cond(uint8(200)), Third3[float32](), Third3[float64]())
	const typed int8 = 5
	fmt.Println(Twice(typed), Offset(typed), Big/(1<<38), Offset(int64(Big)))
}

// Output:
// 7 7 7 7 7 7
// -56 144 4.5 2.2 2.5 2.5
// [1 4 7 10 13] [1 4 7 10] [1 4 7] 0
// -106 44 100.5 9223372036854775908
// 7 7 7 127 127 127
// 11 15 3.5 2.5
// seven big small seven big 0.33333334 0.3333333333333333
// 10 105 4 1099511627876
