// phase: 07

// Variadic generic functions: no arguments, several, and a slice spread with ...
package main

import "fmt"

func Of[T any](vs ...T) []T { return vs }

func Sum[T ~int | ~float64](vs ...T) T {
	var s T
	for _, v := range vs {
		s += v
	}
	return s
}

func Concat[T any](lists ...[]T) []T {
	var out []T
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

func Prefix[T any](first T, rest ...T) []T { return append([]T{first}, rest...) }

func CallAll[T any](f func(T) string, vs ...T) []string {
	var out []string
	for _, v := range vs {
		out = append(out, f(v))
	}
	return out
}

func Printf[T any](format string, vs ...T) string {
	args := make([]any, len(vs))
	for i, v := range vs {
		args[i] = v
	}
	return fmt.Sprintf(format, args...)
}

func Forward[T any](vs ...T) int { return len(Of(vs...)) }

func main() {
	fmt.Println(Of(1, 2, 3), len(Of[string]()), Of[string]() == nil, Of("a"))
	fmt.Println(Sum(1, 2, 3), Sum(1.5, 2.5), Sum[int](), Sum([]int{4, 5}...))
	fmt.Println(Concat([]int{1}, []int{2, 3}, nil), Concat[string]() == nil, Concat([]string{"x"}))
	fmt.Println(Prefix(1), Prefix("a", "b", "c"), Prefix(1.5, []float64{2.5}...))
	fmt.Println(CallAll(func(i int) string { return fmt.Sprint(i * 2) }, 1, 2, 3))
	fmt.Println(Printf("%d-%d", 1, 2), Printf("%s", "only"), Printf[any]("%v %v", 1, "x"))
	fmt.Println(Forward(1, 2), Forward[int](), Forward([]string{"a", "b", "c"}...))
}

// Output:
// [1 2 3] 0 true [a]
// 6 4 0 9
// [1 2 3] true [x]
// [1] [a b c] [1.5 2.5]
// [2 4 6]
// 1-2 only 1 x
// 2 0 3
