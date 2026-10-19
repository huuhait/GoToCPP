// phase: 08

// A type parameter of a func may be named C, as in Go code that composes funcs or makes channels.
package main

import "fmt"

func Compose[A, B, C any](f func(A) B, g func(B) C) func(A) C {
	return func(a A) C { return g(f(a)) }
}

func MkChan[C ~chan E, E any](n int, vs ...E) C {
	ch := make(C, n)
	for _, v := range vs {
		ch <- v
	}
	return ch
}

func Last[C ~[]E, E any](c C) E { return c[len(c)-1] }

func main() {
	f := Compose(func(i int) float64 { return float64(i) / 2 }, func(x float64) string { return fmt.Sprint(x) })
	fmt.Println(f(3), f(8))
	ch := MkChan[chan string](2, "a", "b")
	fmt.Println(len(ch), cap(ch), <-ch)
	fmt.Println(Last([]int{1, 2, 3}))
}

// Output:
// 1.5 4
// 2 2 a
// 3
