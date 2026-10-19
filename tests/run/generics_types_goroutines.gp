// phase: 08

// Methods of generic types called from goroutines: a mutex-free counter per goroutine, results by channel.
package main

import "fmt"

type Acc[T ~int | ~float64] struct{ sum T }

func (a *Acc[T]) Add(v T) { a.sum += v }

func SumAll[T ~int | ~float64](parts [][]T) T {
	done := make(chan T)
	for _, p := range parts {
		go func() {
			var a Acc[T]
			for _, v := range p {
				a.Add(v)
			}
			done <- a.sum
		}()
	}
	var total Acc[T]
	for range parts {
		total.Add(<-done)
	}
	return total.sum
}

type Result[T any] struct {
	Val T
	Idx int
}

func Parallel[T, U any](xs []T, f func(T) U) []U {
	ch := make(chan Result[U])
	for i, x := range xs {
		go func() { ch <- Result[U]{f(x), i} }()
	}
	out := make([]U, len(xs))
	for range xs {
		r := <-ch
		out[r.Idx] = r.Val
	}
	return out
}

func main() {
	fmt.Println(SumAll([][]int{{1, 2}, {3}, {4, 5, 6}}), SumAll([][]float64{{0.5}, {0.25, 0.25}}))
	fmt.Println(Parallel([]int{1, 2, 3}, func(i int) string { return fmt.Sprint(i * i) }))
	fmt.Println(Parallel([]string{"a", "bb"}, func(s string) int { return len(s) }))
}

// Output:
// 21 1
// [1 4 9]
// [1 2]
