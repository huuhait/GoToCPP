// phase: 07

// Small integers in generic code: an arithmetic shift of a negative value for every signed width and a
// runtime count, and sums and products that must be cast back to T before they are compared or
// widened (int8 and uint8 wrap before `>`, `==`, division and conversion).
package main

import "fmt"

type Signed interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~int
}

type Small interface{ ~int8 | ~uint8 | ~int16 | ~uint16 }

func Shr[T Signed](x T, n uint) T   { return x >> n }
func ShrI[T Signed](x T, n int) T   { return x >> n }
func ShrEq[T Signed](x T, n uint) T { x >>= n; return x }
func ShrU[T ~uint8 | ~uint16 | ~uint32 | ~uint64](x T, n uint) T { return x >> n }

func Gt[T Small](a, b, c T) bool          { return a+b > c }
func Eq[T Small](a, b, c T) bool          { return a*b == c }
func Wide[T Small](a, b T) int            { return int(a + b) }
func Wide64[T Small](a, b T) int64        { return int64(a * b) }
func Quot[T Small](a, b, c T) T           { return (a + b) / c }
func Neg[T Small](a T) int                { return int(-a) }
func Cmp[T Small](a, b T) bool            { return a+1 < b }
func Float[T Small](a, b T) float64       { return float64(a+b) / 2 }
func Idx[T ~uint8](a, b T) int            { return []int{0, 1, 2, 3}[(a+b)&3] }
func Str[T Small](a, b T) string          { return fmt.Sprint(a+b, a-b, a*b) }

func main() {
	fmt.Println(Shr(int8(-8), 1), Shr(int16(-8), 1), Shr(int32(-8), 1), Shr(int64(-8), 1), Shr(-8, 1))
	fmt.Println(Shr(int8(-128), 7), Shr(int16(-32768), 15), Shr(int32(-1), 31), Shr(int64(-1), 63), Shr(int8(-1), 9))
	fmt.Println(ShrI(int8(-7), 2), ShrI(int16(-7), 3), ShrI(int32(-1<<30), 29), ShrI(int64(-5), 1), ShrEq(int8(-100), 3), ShrEq(int16(-1000), 4))
	fmt.Println(ShrU(uint8(200), 3), ShrU(uint16(65535), 9), ShrU(uint32(1<<31), 31), ShrU(uint64(1<<63), 62))
	fmt.Println(Gt(int8(100), 50, 10), Gt(uint8(200), 100, 10), Gt(int16(30000), 10000, 5), Gt(uint16(60000), 6000, 1))
	fmt.Println(Eq(int8(16), 16, 0), Eq(uint8(16), 16, 0), Eq(int16(256), 256, 0), Eq(uint16(256), 256, 0))
	fmt.Println(Wide(int8(100), 100), Wide(uint8(200), 100), Wide(int16(30000), 30000), Wide(uint16(65535), 2))
	fmt.Println(Wide64(int8(100), 3), Wide64(uint8(200), 3), Wide64(int16(300), 300), Wide64(uint16(500), 500))
	fmt.Println(Quot(int8(100), 100, 3), Quot(uint8(200), 100, 7), Quot(int16(30000), 30000, 11), Quot(uint16(65535), 3, 5))
	fmt.Println(Neg(int8(-128)), Neg(uint8(1)), Neg(int16(-32768)), Neg(uint16(1)))
	fmt.Println(Cmp(int8(127), 0), Cmp(uint8(255), 3), Cmp(int16(32767), 0), Cmp(uint16(65535), 3))
	fmt.Println(Float(int8(100), 100), Float(uint8(200), 100), Float(int16(30000), 30000), Float(uint16(65535), 5))
	fmt.Println(Idx(uint8(200), 100), Idx(uint8(255), 2), Str(int8(100), 100), Str(uint8(1), 2), Str(uint16(65535), 2))
}

// Output:
// -4 -4 -4 -4 -4
// -1 -1 -1 -1 -1
// -2 -1 -2 -3 -13 -63
// 25 127 1 2
// false true false true
// true true true true
// -56 44 -5536 1
// 44 88 24464 53392
// -18 6 -503 0
// -128 255 -32768 65535
// true true true true
// -28 22 -2768 2
// 0 1 -56 0 16 3 255 2 1 65533 65534
