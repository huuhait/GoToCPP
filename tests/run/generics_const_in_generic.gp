// phase: 07

// Untyped constants in generic code: converted to the type argument at each use (T(1<<3),
// x*2+1, ^T(0), shifts of constants, mixed int/float constants), the exactness of float
// constants for float32 and float64, and the wrap-around of the computation at the type's width.
package main

import "fmt"

type Int interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~int | ~uint
}
type Float interface{ ~float32 | ~float64 }
type Num interface{ Int | Float }

const (
	Big   = 1 << 40
	Third = 1.0 / 3
	Mask  = 0x70
)

func Lin[T Num](x T) T { return x*2 + 1 }

func Ones[T Int]() T { return ^T(0) }

func Shifted[T Int](x T) T { return x<<3 | 1<<2 }

func Half[T Float](x T) T { return x / 2 }

func ThirdOf[T Float](x T) T { return x * Third }

func Masked[T Int](x T) T { return x & Mask }

func Scale[T Num](x T) T { return x * 3 / 2 }

func Offsets[T Num]() [4]T { return [4]T{0, 1, 10, 100} }

type SInt interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~int
}

func Sat[T SInt](x T) T {
	if x > 100 {
		return 100
	}
	if x < -100 {
		return -100
	}
	return x
}

func Pow2[T Int](n int) T { return T(1) << n }

func Frac[T Float]() (T, T, T) { return 0.1, 1e-10, 16777217 }

func Eps[T Float]() T { return 1.0 / (1 << 24) }

func Mod[T Int](x T) T { return x % 7 }

func main() {
	fmt.Println(Lin[int8](100), Lin[uint8](200), Lin[int16](20000), Lin[float32](0.1), Lin[float64](0.1), Lin(int64(1)<<62))
	fmt.Println(Ones[int8](), Ones[uint8](), Ones[int16](), Ones[uint16](), Ones[int32](), Ones[uint32](), Ones[int64](), Ones[uint64](), Ones[uint]())
	fmt.Println(Shifted[int8](20), Shifted[uint8](40), Shifted[int16](10000), Shifted[uint32](1<<30), Shifted[int](5))
	fmt.Println(Half[float32](3), Half[float64](3), Half[float32](1e-45))
	fmt.Println(ThirdOf[float32](3), ThirdOf[float64](3), ThirdOf[float32](1))
	fmt.Println(Masked[int8](-1), Masked[uint8](255), Masked[int16](0x1FF), Masked[uint64](Big+0xFF))
	fmt.Println(Scale[int8](100), Scale[uint8](200), Scale[int](7), Scale[float64](7), Scale[float32](7))
	fmt.Println(Offsets[int8](), Offsets[float32](), Offsets[uint8]())
	fmt.Println(Sat[int8](127), Sat[int8](-128), Sat[int16](255), Sat[int](50), Sat(int64(-5)))
	fmt.Println(Pow2[int8](6), Pow2[int8](7), Pow2[uint8](7), Pow2[uint8](8), Pow2[int64](62), Pow2[int64](63), Pow2[uint64](63), Pow2[uint64](64), Pow2[int16](15))
	fmt.Println(Frac[float32]())
	fmt.Println(Frac[float64]())
	fmt.Println(Eps[float32]() > 0, Eps[float64](), 1+Eps[float32]() == 1, 1+Eps[float64]() == 1)
	fmt.Println(Mod[int8](-100), Mod[uint8](255), Mod[int16](-32768), Mod(uint(1<<63)))
}

// Output:
// -55 145 -25535 1.2 1.2 -9223372036854775807
// -1 255 -1 65535 -1 4294967295 -1 18446744073709551615 18446744073709551615
// -92 68 14468 4 44
// 1.5 1.5 0
// 1 1 0.33333334
// 112 112 112 112
// 22 44 10 10.5 10.5
// [0 1 10 100] [0 1 10 100] [0 1 10 100]
// 100 -100 100 50 -5
// 64 -128 128 0 4611686018427387904 -9223372036854775808 9223372036854775808 0 -32768
// 0.1 1e-10 1.6777216e+07
// 0.1 1e-10 1.6777217e+07
// true 5.960464477539063e-08 true false
// -2 3 -1 1
