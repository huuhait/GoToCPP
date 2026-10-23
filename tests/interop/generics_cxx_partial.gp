// phase: 08

// Type arguments that are partly written and partly deduced by C++ (Mixed[int](7, words) with words a []std.string):
// go/types instantiates the function with the written int and an invalid T, so the instance is opaque, not its generic
// name. In a call that is printed, in an expression, in :=, in var with a type, in defer and in a goroutine, with
// two written type arguments, with a deduced map element, and with a deduced type that is the result.
package main

import (
	"fmt"

	"<string>"
	"<vector>"
)

func Mixed[N any, T any](n N, xs []T) int {
	c := 0
	for range xs {
		c++
	}
	fmt.Println("n", n)
	return c
}

func First[N any, T any](n N, xs []T) T {
	return xs[0]
}

func Three[A, B any, T any](a A, b B, xs []T) (A, int) {
	return a, len(xs)
}

func Pair[N any, K comparable, V any](n N, m map[K]V) int {
	return len(m) + 0*len(fmt.Sprint(n))
}

func main() {
	words := []std.string{"a", "bcd"}
	fmt.Println(Mixed[int](7, words))
	fmt.Println(int(First[string]("x", words).size()) + Mixed[int8](1, words))
	f := First[int](1, words)
	fmt.Println(int(f.size()))
	a, n := Three[int, string](1, "s", words)
	fmt.Println(a, n)
	m := map[int]std.string{1: "x"}
	fmt.Println(Pair[bool](true, m))
	vs := [][]std.vector[int]{{}, {}}
	fmt.Println(Mixed[float64](1.5, vs))
	var total int = Mixed[int](10, words)
	fmt.Println(total)
	done := make(chan int)
	go func() {
		done <- Mixed[uint8](9, words)
	}()
	fmt.Println(<-done)
	defer Mixed[int](8, words)
}

// Output:
// n 7
// 2
// n 1
// 3
// 1
// 1 2
// 1
// n 1.5
// 2
// n 10
// 2
// n 9
// 2
// n 8
