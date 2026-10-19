// phase: 07

// Arithmetic and bit operations on every integer type of a type set: wrap-around at each width.
package main

import "fmt"

type Int interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~int | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uint | ~uintptr
}

type MyU8 uint8
type MyI16 int16

func Add[T Int](a, b T) T    { return a + b }
func Sub[T Int](a, b T) T    { return a - b }
func Mul[T Int](a, b T) T    { return a * b }
func Neg[T Int](a T) T       { return -a }
func Not[T Int](a T) T       { return ^a }
func And[T Int](a, b T) T    { return a & b }
func Or[T Int](a, b T) T     { return a | b }
func Xor[T Int](a, b T) T    { return a ^ b }
func AndNot[T Int](a, b T) T { return a &^ b }
func Inc[T Int](a T) T       { a++; return a }
func Dec[T Int](a T) T       { a--; return a }
func Poly[T Int](x T) T      { return x*x*x - 3*x + 7 }

func row[T Int](name string, a, b T) {
	fmt.Println(name, Add(a, b), Sub(a, b), Mul(a, b), Neg(a), Not(a), And(a, b), Or(a, b), Xor(a, b), AndNot(a, b), Inc(a), Dec(a), Poly(a))
}

func main() {
	row[int8]("i8", 100, 50)
	row[int8]("i8", -128, 1)
	row[int16]("i16", 30000, 10000)
	row[int32]("i32", 2000000000, 2000000000)
	row[int64]("i64", 1<<62, 1<<62)
	row[int]("int", -1<<63, -1)
	row[uint8]("u8", 200, 100)
	row[uint8]("u8", 0, 1)
	row[uint16]("u16", 65535, 2)
	row[uint32]("u32", 4000000000, 500000000)
	row[uint64]("u64", 1<<63, 1<<63)
	row[uint]("uint", 0, 5)
	row[uintptr]("uptr", 1, 2)
	row[MyU8]("MyU8", 255, 255)
	row[MyI16]("MyI16", -32768, -32768)
	var x uint8 = 250
	for i := 0; i < 10; i++ {
		x = Inc(x)
	}
	fmt.Println(x, Dec(Dec(uint8(1))), Poly(uint8(7)), Poly(int8(7)))
}

// Output:
// i8 -106 50 -120 -100 -101 32 118 86 68 101 99 27
// i8 -127 127 -128 -128 127 0 -127 -127 -128 -127 127 -121
// i16 -25536 20000 -23808 -30000 -30001 9488 30512 21024 20512 30001 29999 20599
// i32 -294967296 0 -1651507200 -2000000000 -2000000001 2000000000 2000000000 0 0 2000000001 1999999999 -631290873
// i64 -9223372036854775808 0 0 -4611686018427387904 -4611686018427387905 4611686018427387904 4611686018427387904 0 0 4611686018427387905 4611686018427387903 4611686018427387911
// int 9223372036854775807 -9223372036854775807 -9223372036854775808 -9223372036854775808 9223372036854775807 -9223372036854775808 -1 9223372036854775807 0 -9223372036854775807 9223372036854775807 -9223372036854775801
// u8 44 100 32 56 55 64 236 172 136 201 199 175
// u8 1 255 0 0 255 0 1 1 0 1 255 7
// u16 1 65533 65534 1 0 2 65535 65533 65533 0 65534 9
// u32 205032704 3500000000 1321730048 294967296 294967295 206118912 4293881088 4087762176 3793881088 4000000001 3999999999 884901895
// u64 0 0 0 9223372036854775808 9223372036854775807 9223372036854775808 9223372036854775808 0 0 9223372036854775809 9223372036854775807 9223372036854775815
// uint 5 18446744073709551611 0 0 18446744073709551615 0 5 5 0 1 18446744073709551615 7
// uptr 3 18446744073709551615 2 18446744073709551615 18446744073709551614 0 3 3 1 2 0 5
// MyU8 254 0 1 1 0 255 255 0 0 0 254 9
// MyI16 0 0 0 -32768 32767 -32768 -32768 0 0 -32767 32767 -32761
// 4 255 73 73
