// phase: 07

// Generic functions calling each other: inferred and explicit arguments forwarded, T and []T mixed.
package main

import "fmt"

type Number interface{ ~int | ~float64 }

func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func Filter[T any](xs []T, p func(T) bool) []T {
	var out []T
	for _, x := range xs {
		if p(x) {
			out = append(out, x)
		}
	}
	return out
}

func Reduce[T, A any](xs []T, init A, f func(A, T) A) A {
	for _, x := range xs {
		init = f(init, x)
	}
	return init
}

func Sum[T Number](xs []T) T {
	return Reduce(xs, T(0), func(a, x T) T { return a + x })
}

func SumBy[T any, N Number](xs []T, f func(T) N) N { return Sum(Map(xs, f)) }

func CountIf[T any](xs []T, p func(T) bool) int { return len(Filter(xs, p)) }

func Flatten[T any](xss [][]T) []T {
	return Reduce(xss, []T(nil), func(a []T, xs []T) []T { return append(a, xs...) })
}

func FlatMap[T, U any](xs []T, f func(T) []U) []U { return Flatten(Map(xs, f)) }

func Compose[A, B, R any](f func(A) B, g func(B) R) func(A) R { return func(a A) R { return g(f(a)) } }

func Pipe[T any](v T, fs ...func(T) T) T {
	return Reduce(fs, v, func(a T, f func(T) T) T { return f(a) })
}

func main() {
	words := []string{"go", "gopher", "gc", "cpp"}
	fmt.Println(SumBy(words, func(s string) int { return len(s) }), SumBy(words, func(s string) float64 { return float64(len(s)) / 2 }))
	fmt.Println(CountIf(words, func(s string) bool { return s[0] == 'g' }))
	fmt.Println(Flatten([][]int{{1}, {2, 3}, nil}), FlatMap(words, func(s string) []byte { return []byte(s[:2]) }))
	up := Compose(func(s string) int { return len(s) }, func(n int) []int { return make([]int, n) })
	fmt.Println(len(up("abcd")), Compose(up, func(xs []int) bool { return len(xs) > 2 })("ab"))
	fmt.Println(Pipe(2, func(i int) int { return i + 1 }, func(i int) int { return i * 10 }), Pipe("x"))
	fmt.Println(Sum(Map(Filter([]int{1, 2, 3, 4, 5, 6}, func(i int) bool { return i%2 == 0 }), func(i int) float64 { return float64(i) * 1.5 })))
}

// Output:
// 13 6.5
// 3
// [1 2 3] [103 111 103 111 103 99 99 112]
// 4 false
// 30 x
// 18
