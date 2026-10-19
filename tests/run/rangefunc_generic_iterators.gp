// phase: 07

package main

import "fmt"

func Values[V any](s []V) func(yield func(V) bool) {
	return func(yield func(V) bool) {
		for _, v := range s {
			if !yield(v) {
				return
			}
		}
	}
}

func Map[T, U any](seq func(yield func(T) bool), f func(T) U) func(yield func(U) bool) {
	return func(yield func(U) bool) {
		for x := range seq {
			if !yield(f(x)) {
				return
			}
		}
	}
}

func Filter[T any](seq func(yield func(T) bool), keep func(T) bool) func(yield func(T) bool) {
	return func(yield func(T) bool) {
		seq(func(x T) bool {
			if keep(x) {
				return yield(x)
			}
			return true
		})
	}
}

func Sum[T int | float64](seq func(yield func(T) bool)) T {
	var total T
	for x := range seq {
		total += x
	}
	return total
}

func First[T any](seq func(yield func(T) bool), def T) T {
	for x := range seq {
		return x
	}
	return def
}

func Enumerate[T any](s []T) func(yield func(int, T) bool) {
	return func(yield func(int, T) bool) {
		for i, v := range s {
			if !yield(i, v) {
				return
			}
		}
	}
}

func Collect[T any](seq func(yield func(T) bool)) []T {
	var out []T
	for x := range seq {
		out = append(out, x)
	}
	return out
}

func Zip[A, B any](as []A, bs []B) func(yield func(A, B) bool) {
	return func(yield func(A, B) bool) {
		for i := range min(len(as), len(bs)) {
			if !yield(as[i], bs[i]) {
				return
			}
		}
	}
}

func main() {
	nums := []int{1, 2, 3, 4, 5, 6}
	fmt.Println(Sum(Values(nums)))
	fmt.Println(Sum(Filter(Values(nums), func(x int) bool { return x%2 == 0 })))
	fmt.Println(Collect(Map(Values(nums), func(x int) string { return fmt.Sprint("<", x, ">") })))
	fmt.Println(First(Values([]string{}), "default"), First(Values([]string{"x"}), "default"))
	fmt.Println(Sum(Values([]float64{1.5, 2.5})))
	for i, s := range Enumerate([]string{"a", "b"}) {
		fmt.Println(i, s)
	}
	for a, b := range Zip([]int{1, 2, 3}, []string{"x", "y"}) {
		fmt.Println(a, b)
	}
}

// Output:
// 21
// 12
// [<1> <2> <3> <4> <5> <6>]
// default x
// 4
// 0 a
// 1 b
// 1 x
// 2 y
