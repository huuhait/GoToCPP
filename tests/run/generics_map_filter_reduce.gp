// phase: 07

// Map, Filter and Reduce: type arguments inferred from the arguments, including func values.
package main

import "fmt"

func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func Filter[T any](xs []T, keep func(T) bool) []T {
	var out []T
	for _, x := range xs {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}

func Reduce[T, A any](xs []T, init A, f func(A, T) A) A {
	acc := init
	for _, x := range xs {
		acc = f(acc, x)
	}
	return acc
}

func itoa(i int) string { return fmt.Sprint(i) }

func main() {
	names := Map([]int{1, 2, 3}, itoa)
	fmt.Println(names, len(names))
	evens := Filter([]int{1, 2, 3, 4, 5, 6}, func(i int) bool { return i%2 == 0 })
	fmt.Println(evens)
	total := Reduce(evens, 0, func(a, x int) int { return a + x })
	fmt.Println(total)
	words := Reduce([]string{"a", "b", "c"}, "", func(a string, s string) string { return a + s })
	fmt.Println(words)
	lens := Map(Map([]float64{1.5, 22.25}, func(f float64) string { return fmt.Sprint(f) }), func(s string) int { return len(s) })
	fmt.Println(lens)
	fmt.Println(Filter([]string(nil), func(string) bool { return true }) == nil)
}

// Output:
// [1 2 3] 3
// [2 4 6]
// 12
// abc
// [3 5]
// true
