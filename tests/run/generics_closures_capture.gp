// phase: 07

// Closures inside generic functions capture values of the type parameters, and share variables.
package main

import "fmt"

func Counter[T ~int | ~int8 | ~float64](start, step T) func() T {
	c := start
	return func() T {
		c += step
		return c
	}
}

func Memo[K comparable, V any](f func(K) V) func(K) V {
	cache := map[K]V{}
	return func(k K) V {
		if v, ok := cache[k]; ok {
			return v
		}
		v := f(k)
		cache[k] = v
		return v
	}
}

func Funcs[T any](vs []T) []func() T {
	var out []func() T
	for _, v := range vs {
		out = append(out, func() T { return v })
	}
	return out
}

func Pair[T any](v T) (get func() T, set func(T)) {
	get = func() T { return v }
	set = func(n T) { v = n }
	return
}

func Compose[A, B, R any](f func(A) B, g func(B) R) func(A) R {
	return func(a A) R { return g(f(a)) }
}

func main() {
	c1 := Counter(10, 5)
	fmt.Println(c1(), c1(), c1())
	c2 := Counter[int8](125, 1)
	fmt.Println(c2(), c2(), c2(), c2())
	c3 := Counter(0.5, 0.25)
	fmt.Println(c3(), c3())
	calls := 0
	sq := Memo(func(n int) int { calls++; return n * n })
	fmt.Println(sq(4), sq(4), sq(5), calls)
	for _, f := range Funcs([]string{"x", "y", "z"}) {
		fmt.Print(f())
	}
	fmt.Println()
	get, set := Pair("a")
	set(get() + "b")
	fmt.Println(get())
	h := Compose(func(i int) float64 { return float64(i) / 2 }, func(f float64) string { return fmt.Sprint(f) })
	fmt.Println(h(5), h(8))
}

// Output:
// 15 20 25
// 126 127 -128 -127
// 0.75 1
// 16 16 25 2
// xyz
// ab
// 2.5 4
