// phase: 07

// Variadic generic functions: ...T with no arguments, spread of named slice types, a slice of
// interfaces, append with a spread of a different defined slice type, nil spreads, and a call
// f(g()) where g returns several values for the variadic tail.
package main

import "fmt"

type Ints []int
type Strs []string

func Count[T any](xs ...T) (int, bool) { return len(xs), xs == nil }

func Prepend[S ~[]E, E any](s S, es ...E) S { return append(es, s...) }

func Concat[S ~[]E, E any](ss ...S) S {
	var out S
	for _, s := range ss {
		out = append(out, s...)
	}
	return out
}

func Pairs[K comparable, V any](kv ...any) map[K]V {
	m := map[K]V{}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i].(K)] = kv[i+1].(V)
	}
	return m
}

func Tail[T any](first T, rest ...T) []T { return rest }

func two() (int, int)                 { return 1, 2 }
func three() (string, string, string) { return "a", "b", "c" }

func Sum[T ~int | ~float64](xs ...T) (s T) {
	for _, x := range xs {
		s += x
	}
	return
}

func Apply[T any](f func(...T) int, xs ...T) int { return f(xs...) }

func main() {
	fmt.Println(Count[int]())
	fmt.Println(Count[string]("a"))
	fmt.Println(Count[int](nil...))
	fmt.Println(Count([]int{}...))
	fmt.Println(Count[any](nil, nil))
	fmt.Println(Count[Ints](Ints{1}, nil))
	is := Ints{3, 4}
	fmt.Println(Prepend(is, 1, 2), Prepend(is), Prepend(Strs{"z"}, "x", "y"))
	fmt.Printf("%T %T\n", Prepend(is, 0), Prepend([]string(nil), "q"))
	fmt.Println(Concat(is, Ints{5}, nil, Ints{6, 7}), Concat[Strs](), Concat([]int{1}, []int{2}))
	fmt.Println(len(Concat[Ints]()), Concat[Ints]() == nil)
	fmt.Println(Pairs[string, int]("a", 1, "b", 2, "c"))
	fmt.Println(Pairs[int, []string](1, []string{"x"}))
	fmt.Println(Tail(1), Tail(1, 2, 3), Tail("s", is2()...), len(Tail(0)))
	fmt.Println(Sum(two()), Sum[float64](1.5, 2.5), Sum[Ints2]())
	fmt.Println(Tail(three()), Apply(func(xs ...string) int { return len(xs) }, "x", "y"))
	fmt.Println(Apply(func(xs ...int) int { return len(xs) * 10 }))
}

type Ints2 int

func is2() []string { return []string{"t", "u"} }

// Output:
// 0 true
// 1 false
// 0 true
// 0 false
// 2 false
// 2 false
// [1 2 3 4] [3 4] [x y z]
// main.Ints []string
// [3 4 5 6 7] [] [1 2]
// 0 true
// map[a:1 b:2]
// map[1:[x]]
// [] [2 3] [t u] 0
// 3 4 0
// [b c] 2
// 0
