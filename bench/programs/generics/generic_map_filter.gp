// bench: generic_map_filter
// category: generics
// size: 3000
// smoke: 20
// twin: yes
// about: generic Map, Filter and Reduce over []int, []float64 and []string (three instances of each)
package main

import "fmt"

func Map[T, U any](xs []T, f func(T) U) []U {
	out := make([]U, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func Filter[T any](xs []T, keep func(T) bool) []T {
	var out []T
	for _, x := range xs {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}

func Reduce[T, A any](xs []T, init A, f func(A, T) A) A {
	acc := init
	for _, x := range xs {
		acc = f(acc, x)
	}
	return acc
}

func main() {
	ints := make([]int, 10000)
	floats := make([]float64, 10000)
	strs := make([]string, 1000)
	for i := range ints {
		ints[i] = i
		floats[i] = float64(i) * 0.5
	}
	for i := range strs {
		strs[i] = fmt.Sprint(i)
	}
	isum, fsum, ssum := 0, 0.0, 0
	for r := 0; r < benchN; r++ {
		sq := Map(ints, func(x int) int { return x*x + r })
		ev := Filter(sq, func(x int) bool { return x%2 == 0 })
		isum += Reduce(ev, 0, func(a, x int) int { return a + x%1000 })
		hf := Map(floats, func(x float64) float64 { return x*0.25 + 1 })
		pos := Filter(hf, func(x float64) bool { return x > 100 })
		fsum += Reduce(pos, 0.0, func(a, x float64) float64 { return a + x })
		ls := Map(strs, func(s string) int { return len(s) + r })
		ssum += Reduce(Filter(ls, func(n int) bool { return n > 3 }), 0, func(a, n int) int { return a + n })
	}
	fmt.Printf("%d %.3f %d\n", isum, fsum, ssum)
}
