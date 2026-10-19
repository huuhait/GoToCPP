// phase: 07

// A generic type whose underlying type is a func, with methods, passed and combined.
package main

import "fmt"

type Handler[T any] func(T) T

func (h Handler[T]) Twice(v T) T { return h(h(v)) }

func (h Handler[T]) Then(next Handler[T]) Handler[T] {
	return func(v T) T { return next(h(v)) }
}

type Pred[T any] func(T) bool

func (p Pred[T]) Not() Pred[T]            { return func(v T) bool { return !p(v) } }
func (p Pred[T]) And(q Pred[T]) Pred[T]   { return func(v T) bool { return p(v) && q(v) } }

func Filter[T any](xs []T, p Pred[T]) []T {
	var out []T
	for _, x := range xs {
		if p(x) {
			out = append(out, x)
		}
	}
	return out
}

func main() {
	inc := Handler[int](func(i int) int { return i + 1 })
	dbl := Handler[int](func(i int) int { return i * 2 })
	fmt.Println(inc.Twice(1), inc.Then(dbl)(3), dbl.Then(inc).Twice(1))
	up := Handler[string](func(s string) string { return s + "!" })
	fmt.Println(up.Twice("hi"))
	even := Pred[int](func(i int) bool { return i%2 == 0 })
	big := Pred[int](func(i int) bool { return i > 3 })
	xs := []int{1, 2, 3, 4, 5, 6}
	fmt.Println(Filter(xs, even), Filter(xs, even.Not()), Filter(xs, even.And(big)))
	var nilH Handler[float64]
	fmt.Println(nilH == nil)
	fmt.Printf("%T %T\n", inc, even.Not())
}

// Output:
// 3 8 7
// hi!!
// [2 4 6] [1 3 5] [4 6]
// true
// main.Handler[int] main.Pred[int]
