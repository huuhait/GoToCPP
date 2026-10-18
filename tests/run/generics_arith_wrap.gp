// phase: 07

// Arithmetic on small integer type arguments wraps around like Go's, not like C++'s int promotion.
package main

import "fmt"

type Int interface {
	~int8 | ~uint8 | ~int16 | ~uint16 | ~int32 | ~uint32 | ~int64 | ~uint64 | ~int | ~uint
}

type Small int8

func Add[T Int](a, b T) T { return a + b }
func Sub[T Int](a, b T) T { return a - b }
func Mul[T Int](a, b T) T { return a * b }
func Neg[T Int](a T) T    { return -a }
func Inc[T Int](a T) T {
	a++
	return a
}
func Dec[T Int](a T) T {
	a--
	return a
}
func AddAssign[T Int](a, b T) T {
	a += b
	a *= 2
	return a
}
func Not[T Int](a T) T { return ^a }
func Mix[T Int](a, b T) T { return (a&b | a^b) &^ 3 }
func Positive[T Int](a, b T) bool { return a+b > 0 }

func main() {
	fmt.Println(Add[int8](100, 100), Add[uint8](200, 100), Add[int16](32767, 1), Add[uint16](65535, 1))
	fmt.Println(Add[int32](2147483647, 1), Add[uint32](4294967295, 1), Add[int64](9223372036854775807, 1), Add[uint64](18446744073709551615, 2))
	fmt.Println(Sub[uint8](1, 2), Sub[int8](-128, 1), Sub[uint64](0, 1), Sub[uint](3, 5) > 0)
	fmt.Println(Mul[int8](16, 16), Mul[uint8](16, 17), Mul[int16](300, 300), Mul[int32](65536, 65536), Mul[int](1<<40, 1<<40))
	fmt.Println(Neg[int8](-128), Neg[uint8](1), Neg[Small](127), Neg[uint16](1))
	fmt.Println(Inc[int8](127), Inc[uint8](255), Dec[uint8](0), Dec[int8](-128), Inc[Small](127))
	fmt.Println(AddAssign[int8](60, 10), AddAssign[uint8](100, 30), AddAssign[Small](1, 1))
	fmt.Println(Not[int8](0), Not[uint8](0), Not[uint16](255), Not[Small](-1))
	fmt.Println(Mix[int8](0x55, 0x3c), Mix[uint8](0xf0, 0x1f), Mix[Small](5, 6))
	fmt.Println(Positive[int8](100, 100), Positive[int8](100, 27), Positive[int](100, 100))
}

// Output:
// -56 44 -32768 0
// -2147483648 0 -9223372036854775808 1
// 255 127 18446744073709551615 true
// 0 16 24464 0 0
// -128 255 -127 65535
// -128 0 255 127 -128
// -116 4 4
// -1 255 65280 0
// 104 236 0
// false true true
