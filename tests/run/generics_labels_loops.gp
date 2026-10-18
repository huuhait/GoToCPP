// phase: 07

// break, continue and goto with labels, switch and select-free control flow inside generic loops.
package main

import "fmt"

type Int interface{ ~int | ~int8 | ~uint8 }

func Find[T comparable](grid [][]T, v T) (int, int) {
outer:
	for i, row := range grid {
		for j, x := range row {
			if x == v {
				return i, j
			}
			if j > 2 {
				continue outer
			}
		}
	}
	return -1, -1
}

func Pairs[T Int](n T) int {
	count := 0
outer:
	for i := T(0); i < n; i++ {
		for j := T(0); j < n; j++ {
			switch {
			case j > i:
				continue outer
			case i+j > 10:
				break outer
			}
			count++
		}
	}
	return count
}

func Retry[T any](f func(int) (T, bool)) (T, int) {
	attempt := 0
retry:
	attempt++
	if v, ok := f(attempt); ok {
		return v, attempt
	}
	if attempt < 5 {
		goto retry
	}
	var zero T
	return zero, attempt
}

func Classify[T Int](xs []T) (neg, zero, pos int) {
	for _, x := range xs {
		switch {
		case x < 0:
			neg++
		case x == 0:
			zero++
			fallthrough
		default:
			if x > 0 {
				pos++
			}
		}
	}
	return
}

func main() {
	fmt.Println(Find([][]int{{1, 2}, {3, 4, 5}}, 5))
	fmt.Println(Find([][]string{{"a", "b", "c", "d", "e"}, {"x"}}, "x"))
	fmt.Println(Find([][]int{{1}}, 9))
	fmt.Println(Pairs(5), Pairs[int8](12), Pairs[uint8](3))
	fmt.Println(Retry(func(n int) (string, bool) { return fmt.Sprint("got", n), n == 3 }))
	fmt.Println(Retry(func(n int) (int, bool) { return n, false }))
	fmt.Println(Classify([]int{-1, 0, 2, 0, 3}))
	fmt.Println(Classify([]int8{-5, -6}))
}

// Output:
// 1 2
// 1 0
// -1 -1
// 15 26 6
// got3 3
// 0 5
// 1 2 2
// 2 0 0
