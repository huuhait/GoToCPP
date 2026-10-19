// phase: 07

package main

import "fmt"

func Seq[T any](xs ...T) func(yield func(T) bool) {
	return func(yield func(T) bool) {
		for _, x := range xs {
			if !yield(x) {
				return
			}
		}
	}
}

func Find[T comparable](seq func(yield func(T) bool), want T) (int, bool) {
	i := 0
	for x := range seq {
		if x == want {
			return i, true
		}
		i++
	}
	return -1, false
}

func Max[T int | string | float64](seq func(yield func(T) bool)) (best T, ok bool) {
	for x := range seq {
		if !ok || x > best {
			best, ok = x, true
		}
	}
	return
}

func Nested[T any](a, b []T) (n int) {
outer:
	for x := range Seq(a...) {
		for y := range Seq(b...) {
			n++
			if any(x) == any(y) {
				continue outer
			}
		}
	}
	return n
}

type Number interface{ ~int | ~int8 }

type Small int8

func Total[T Number](seq func(yield func(T) bool)) (t T) {
	for x := range seq {
		t += x
	}
	return
}

func main() {
	fmt.Println(Find(Seq(3, 4, 5), 5))
	fmt.Println(Find(Seq("a", "b"), "z"))
	fmt.Println(Max(Seq(3, 9, 2)))
	fmt.Println(Max(Seq("pear", "apple", "zebra")))
	fmt.Println(Max(Seq[float64]()))
	fmt.Println(Nested([]int{1, 2, 3}, []int{2, 3}))
	fmt.Println(Total(Seq(Small(100), Small(100))))
	fmt.Println(Total(Seq(1, 2, 3)))
}

// Output:
// 2 true
// -1 false
// 9 true
// zebra true
// 0 false
// 5
// -56
// 6
