// phase: 07

// Generic functions and types: the instance names (docs/spec, step 7.1) in backtraces and the
// values that generic functions handle.
package main

import "fmt"

type Pair struct {
	Key string
	Val int
}

type Number interface{ ~int | ~float64 }

func Max[T Number](a, b T) T {
	if a > b {
		return a // @max
	}
	return b
}

func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x)) // @map
	}
	return out
}

func Show[T any](v T) {
	fmt.Println(v) // @show
}

func main() {
	p := Pair{"a", 1}
	fmt.Println(Max(2, 1), Max(2.5, 1.5), p)
	fmt.Println(Map([]int{1, 2}, func(i int) string { return fmt.Sprint(i) }))
	Show(p)
	Show("s")
}
