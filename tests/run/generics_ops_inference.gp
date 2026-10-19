// phase: 07

// Type argument inference: untyped constants, function arguments, core types, partial lists, named types.
package main

import "fmt"

type Num interface{ ~int | ~int8 | ~float32 | ~float64 }

type Temp float64
type IDs []int

func Max[T Num](a, b T) T {
	if a > b {
		return a
	}
	return b
}
func Sum[T Num](xs ...T) (t T) {
	for _, x := range xs {
		t += x
	}
	return
}
func Map[S ~[]E, E, R any](s S, f func(E) R) []R {
	var out []R
	for _, e := range s {
		out = append(out, f(e))
	}
	return out
}
func Conv[To, From Num](x From) To { return To(x) }
func Keys[M ~map[K]V, K comparable, V any](m M) int { return len(m) }
func Apply[T any](f func(T) T, x T) T { return f(x) }
func Pair[A, B any](a A, b B) string { return fmt.Sprintf("%T/%T", a, b) }
func Clone[S ~[]E, E any](s S) S { return append(S(nil), s...) }
func Ptr[T any](x T) *T { return &x }
func Call[F ~func(A) R, A, R any](f F, a A) R { return f(a) }

type Handler func(string) int

func main() {
	fmt.Println(Max(1, 2.5), Max(3, 2), Max(Temp(1), 3), Max[int8](5, 100), Max(1.0, 2))
	fmt.Println(Sum(1, 2, 3), Sum(1.5, 2), Sum[float32](0.1, 0.2), Sum[int8](100, 27, 1), Sum[Temp]())
	fmt.Println(Map(IDs{1, 2}, func(i int) string { return fmt.Sprint(i * 2) }), Map([]string{"a"}, func(s string) int { return len(s) }))
	fmt.Println(Conv[int](2.9), Conv[float32](int8(5)), Conv[Temp](7), Conv[int8, float64](-3.99))
	fmt.Println(Keys(map[string]bool{"a": true}), Keys(map[Temp][]int{}))
	fmt.Println(Apply(func(x int) int { return x * 3 }, 5), Apply(func(s string) string { return s + s }, "ab"))
	fmt.Println(Pair(1, "a"), Pair(1.5, 'x'), Pair(IDs{}, Temp(1)), Pair(true, [2]int{}), Pair(Ptr(1), Ptr[any](nil)))
	fmt.Printf("%T %T %T\n", Clone(IDs{1}), Clone([]string{}), Ptr(Temp(2)))
	fmt.Println(Call(Handler(func(s string) int { return len(s) }), "four"), Call(func(i int) bool { return i > 0 }, 1))
	var h Handler = func(s string) int { return 7 }
	fmt.Println(Call(h, ""))
}

// Output:
// 2.5 3 3 100 2
// 6 3.5 0.3 -128 0
// [2 4] [1]
// 2 5 7 -3
// 1 0
// 15 abab
// int/string float64/int32 main.IDs/main.Temp bool/[2]int *int/*interface {}
// main.IDs []string *main.Temp
// 4 true
// 7
