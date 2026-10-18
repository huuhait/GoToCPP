// phase: 08

// A pipeline of generic stages connected by channels: Map, Filter, Take over goroutines.
package main

import "fmt"

func Source[T any](vs ...T) <-chan T {
	out := make(chan T)
	go func() {
		for _, v := range vs {
			out <- v
		}
		close(out)
	}()
	return out
}

func Map[T, U any](in <-chan T, f func(T) U) <-chan U {
	out := make(chan U)
	go func() {
		for {
			v, ok := <-in
			if !ok {
				close(out)
				return
			}
			out <- f(v)
		}
	}()
	return out
}

func Filter[T any](in <-chan T, keep func(T) bool) <-chan T {
	out := make(chan T)
	go func() {
		for {
			v, ok := <-in
			if !ok {
				close(out)
				return
			}
			if keep(v) {
				out <- v
			}
		}
	}()
	return out
}

func Collect[T any](in <-chan T) []T {
	var out []T
	for {
		v, ok := <-in
		if !ok {
			return out
		}
		out = append(out, v)
	}
}

func main() {
	nums := Source(1, 2, 3, 4, 5, 6, 7, 8)
	sq := Map(nums, func(i int) int { return i * i })
	even := Filter(sq, func(i int) bool { return i%2 == 0 })
	strs := Map(even, func(i int) string { return fmt.Sprint("<", i, ">") })
	fmt.Println(Collect(strs))
	fmt.Println(Collect(Map(Source("a", "bb", "ccc"), func(s string) int { return len(s) })))
	fmt.Println(len(Collect(Filter(Source[float64](), func(float64) bool { return true }))))
}

// Output:
// [<4> <16> <36> <64>]
// [1 2 3]
// 0
