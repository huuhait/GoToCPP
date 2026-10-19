// phase: 07

// Generic functions over small integers: accumulating loops wrap, signed MinInt / -1 and % -1 do not
// trap, division by zero panics (recovered and printed) for every width, shifts by counts of other
// integer types, increments, compound assignments and conversions between the widths.
package main

import "fmt"

type Small interface {
	~int8 | ~uint8 | ~int16 | ~uint16
}

type Int interface {
	Small | ~int32 | ~uint32 | ~int64 | ~uint64 | ~int | ~uint | ~uintptr
}

type Byte uint8
type Short int16

func SumPow[T Int](n T) (s T) {
	for i := T(1); i <= n && i > 0; i++ {
		s += i * i * i
	}
	return
}

func Fact[T Int](n int) T {
	f := T(1)
	for i := 2; i <= n; i++ {
		f *= T(i)
	}
	return f
}

func Fib[T Int](n int) T {
	a, b := T(0), T(1)
	for i := 0; i < n; i++ {
		a, b = b, a+b
	}
	return a
}

func Safe[T Int](f func() T) (v T, msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprint(r)
		}
	}()
	return f(), "ok"
}

func DivMod[T Int](a, b T) (T, T) { return a / b, a % b }

func Shl[T Int, S ~uint8 | ~int | ~uint64](x T, s S) T { return x << s }
func Shr[T Int, S ~uint8 | ~int | ~uint64](x T, s S) T { return x >> s }

func Mix[T, U Int](x T) U { return U(x) }

func Ops[T Int](a, b T) {
	c := a
	c += b
	d := a
	d -= b
	e := a
	e *= b
	f := a
	f <<= 3
	g := a
	g >>= 1
	h := a
	h |= b
	i := a
	i &^= b
	j := a
	j ^= b
	j++
	k := a
	k--
	fmt.Println(c, d, e, f, g, h, i, j, k)
}

func main() {
	fmt.Println(SumPow(int8(10)), SumPow(uint8(10)), SumPow(int16(40)), SumPow(uint16(60)), SumPow(Byte(9)), SumPow(Short(30)))
	fmt.Println(Fact[int8](7), Fact[uint8](7), Fact[int16](9), Fact[uint16](9), Fact[int32](14), Fact[uint32](14), Fact[Byte](10))
	fmt.Println(Fib[int8](13), Fib[uint8](14), Fib[int16](25), Fib[uint16](25), Fib[Short](30))
	fmt.Println(Safe(func() int8 { a, b := DivMod(int8(-128), -1); return a + b }))
	fmt.Println(Safe(func() int8 { a, b := DivMod(int8(7), 0); return a + b }))
	fmt.Println(Safe(func() uint8 { a, b := DivMod(uint8(7), 0); return a + b }))
	fmt.Println(Safe(func() int16 { a, b := DivMod(int16(-32768), -1); return a + b }))
	fmt.Println(Safe(func() uint16 { a, _ := DivMod(uint16(9), 0); return a }))
	fmt.Println(Safe(func() int32 { a, b := DivMod(int32(-2147483648), -1); return a + b }))
	fmt.Println(Safe(func() int64 { a, b := DivMod(int64(-9223372036854775808), -1); return a + b }))
	fmt.Println(Safe(func() uint64 { _, b := DivMod(uint64(5), 0); return b }))
	fmt.Println(Safe(func() Byte { a, _ := DivMod(Byte(200), 7); return a }))
	fmt.Println(Shl(int8(1), 7), Shl(int8(1), uint8(8)), Shl(uint8(255), 1), Shl(int16(-1), uint64(15)), Shl(uint16(1), 16), Shl(int32(1), 31), Shl(uint32(3), 31))
	fmt.Println(Shr(int8(-128), 7), Shr(int8(-128), 8), Shr(int8(-128), 100), Shr(uint8(128), 7), Shr(uint8(128), uint8(200)), Shr(int16(-1), 20), Shr(uint64(1<<63), 63))
	fmt.Println(Safe(func() int8 { n := -1; return Shl(int8(1), n) }))
	fmt.Println(Safe(func() uint16 { n := -5; return Shr(uint16(1), n) }))
	fmt.Println(Mix[int, int8](200), Mix[int, uint8](-1), Mix[uint16, int16](40000), Mix[int16, uint16](-1), Mix[int8, uint64](-1), Mix[uint64, int8](255), Mix[int32, Byte](513), Mix[Short, uint8](-300))
	Ops[int8](100, 100)
	Ops[uint8](200, 100)
	Ops[int16](-30000, 9000)
	Ops[uint16](3, 65535)
	Ops[int32](2000000000, 2000000000)
	Ops[uint64](1, 18446744073709551615)
	Ops[Byte](255, 1)
	Ops[Short](-32768, 1)
}

// Output:
// -47 209 17040 6564 233 19617
// -80 176 -30336 35200 1278945280 1278945280 0
// -23 121 9489 9489 -19928
// -128 ok
// 0 runtime error: integer divide by zero
// 0 runtime error: integer divide by zero
// -32768 ok
// 0 runtime error: integer divide by zero
// -2147483648 ok
// -9223372036854775808 ok
// 0 runtime error: integer divide by zero
// 28 ok
// -128 0 254 -32768 0 -2147483648 2147483648
// -1 -1 -1 1 0 -1 1
// 0 runtime error: negative shift amount
// 0 runtime error: negative shift amount
// -56 255 -25536 65535 18446744073709551615 -1 1 212
// -56 0 16 32 50 100 0 1 99
// 44 100 32 64 100 236 136 173 199
// -21000 26536 8320 22144 -15000 -21512 -30512 -22023 -30001
// 2 4 65533 24 1 65535 0 65533 2
// -294967296 0 -1651507200 -1179869184 1000000000 2000000000 0 1 1999999999
// 0 2 18446744073709551615 8 0 18446744073709551615 0 18446744073709551615 0
// 0 254 255 248 127 255 254 255 254
// -32767 32767 -32768 0 -16384 -32767 -32768 -32766 32767
