// phase: 07

// Explicit type arguments: all of them, some of them (the rest is inferred), and none needed.
package main

import "fmt"

func Convert[To, From ~int | ~int8 | ~float64](x From) To { return To(x) }

func Make[T any](n int) []T { return make([]T, n) }

func Pick[T, U any](t T, u U, first bool) any {
	if first {
		return t
	}
	return u
}

func Zero[T any]() T {
	var z T
	return z
}

type Meters float64

func main() {
	fmt.Println(Convert[int](3.99), Convert[float64](7), Convert[Meters](2))
	fmt.Println(Convert[int, float64](-2.5), Convert[int8, int](300))
	fmt.Println(Make[string](2), len(Make[[]int](3)), Make[bool](1))
	fmt.Println(Pick[int, string](1, "a", true), Pick(1, "a", false), Pick[any, any](nil, 2, true))
	fmt.Println(Zero[int](), Zero[string]() == "", Zero[*int]() == nil, Zero[Meters](), Zero[[2]bool](), Zero[error]() == nil)
	fmt.Printf("%T %T %T\n", Convert[int8](1), Make[int](0), Zero[any]())
}

// Output:
// 3 7 2
// -2 44
// [ ] 3 [false]
// 1 a <nil>
// 0 true true 0 [false false] true
// int8 []int <nil>
