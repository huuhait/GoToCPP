// phase: 07

// Division and remainder by every integer type, with negative operands, MinInt / -1 and zero.
package main

import "fmt"

type Int interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~int | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

func Div[T Int](a, b T) T { return a / b }
func Rem[T Int](a, b T) T { return a % b }

func try[T Int](name string, a, b T) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println(name, a, b, "panic:", r)
		}
	}()
	fmt.Println(name, a, b, Div(a, b), Rem(a, b))
}

func divAssign[T Int](a, b T) T {
	a /= b
	a %= 7
	return a
}

func main() {
	try[int8]("i8", -128, -1)
	try[int8]("i8", -7, 2)
	try[int8]("i8", 7, -2)
	try[int8]("i8", 5, 0)
	try[int16]("i16", -32768, -1)
	try[int16]("i16", -32768, 3)
	try[int32]("i32", -2147483648, -1)
	try[int32]("i32", 100, 0)
	try[int64]("i64", -9223372036854775808, -1)
	try[int64]("i64", -9223372036854775807, 2)
	try[int]("int", -9223372036854775808, -1)
	try[int]("int", 0, 0)
	try[uint8]("u8", 255, 16)
	try[uint8]("u8", 255, 0)
	try[uint16]("u16", 65535, 256)
	try[uint32]("u32", 4294967295, 10)
	try[uint64]("u64", 18446744073709551615, 10)
	try[uint64]("u64", 1, 0)
	fmt.Println(divAssign[int8](-100, 3), divAssign[uint8](200, 3), divAssign(int64(-1000), 9))
}

// Output:
// i8 -128 -1 -128 0
// i8 -7 2 -3 -1
// i8 7 -2 -3 1
// i8 5 0 panic: runtime error: integer divide by zero
// i16 -32768 -1 -32768 0
// i16 -32768 3 -10922 -2
// i32 -2147483648 -1 -2147483648 0
// i32 100 0 panic: runtime error: integer divide by zero
// i64 -9223372036854775808 -1 -9223372036854775808 0
// i64 -9223372036854775807 2 -4611686018427387903 -1
// int -9223372036854775808 -1 -9223372036854775808 0
// int 0 0 panic: runtime error: integer divide by zero
// u8 255 16 15 15
// u8 255 0 panic: runtime error: integer divide by zero
// u16 65535 256 255 255
// u32 4294967295 10 429496729 5
// u64 18446744073709551615 10 1844674407370955161 5
// u64 1 0 panic: runtime error: integer divide by zero
// -5 3 -6
