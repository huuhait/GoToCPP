package main

import "fmt"

func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

type Big [64]int

// Show has one C++ function for each type argument: with Big (512 bytes) the captured y is boxed,
// with int and string it is copied into the lambda.
func Show[T any](x T) {
	var y T = x
	f := func() T { return y }
	fmt.Println(any(f()))
}

func main() {
	Show(1)
	Show(Big{})
	Show("s")
	fmt.Println(Map([]int{1, 2, 3}, func(i int) string { return fmt.Sprint(i * 2) }))
	fmt.Println(Map([]string{"a"}, func(s string) int { return len(s) }))
}
