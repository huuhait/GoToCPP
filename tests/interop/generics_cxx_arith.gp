// phase: 07

// Operators on a type parameter whose type argument is a C type: C++'s own operators do the work (as they do for C values), constants take part as C++ literals.
package main

import "fmt"

func Add[T ~int | ~int32 | ~float64](a, b T) T { return a + b }

func Larger[T ~int | ~int32](a, b T) T {
	if a > b {
		return a
	}
	return b
}

func Inc[T ~int | ~int32](a T) T {
	a++
	a += 2
	a = -a
	return a + 1
}

func Mix[T ~int | ~int32](a, b T) T { return (a*b - 3) % 7 }

func Eq[T comparable](a, b T) bool { return a == b }

func main() {
	fmt.Println(Add[C.double](1.5, 2.5), Add[C.int](40, 2), Larger[C.int](3, 9), Larger[C.int](9, 3))
	fmt.Println(Inc[C.int](5), Mix[C.int](4, 5), Eq[C.int](1, 1), Eq[C.double](1, 2))
}

// Output:
// 4 42 9 9
// -7 3 true false
