// phase: 07
// lsan: cycle

// Closures in generic functions: per-iteration loop variables of type T, closures capturing and
// modifying type-parameter-typed variables and parameters, closures returned from generic
// functions and called after the instance returned, recursive closures, closures stored in
// slices of func(T) T, and generic functions that return generic closures.
package main

import "fmt"

type Num interface{ ~int | ~int8 | ~float64 }

func Counters[T Num](n int) []func() T {
	var fs []func() T
	for i := 0; i < n; i++ {
		var c T
		fs = append(fs, func() T { c += T(i) + 1; return c })
	}
	return fs
}

func Accum[T Num](start T) (add func(T) T, get func() T) {
	total := start
	add = func(d T) T { total += d; return total }
	get = func() T { return total }
	return
}

func Memo[K comparable, V any](f func(K) V) (func(K) V, *int) {
	cache := map[K]V{}
	calls := 0
	return func(k K) V {
		if v, ok := cache[k]; ok {
			return v
		}
		calls++
		v := f(k)
		cache[k] = v
		return v
	}, &calls
}

func Fix[T any](f func(rec func(T) T, x T) T) func(T) T {
	var rec func(T) T
	rec = func(x T) T { return f(rec, x) }
	return rec
}

func Compose[A, B, C any](f func(A) B, g func(B) C) func(A) C { return func(a A) C { return g(f(a)) } }

func Curry[A, B, R any](f func(A, B) R) func(A) func(B) R {
	return func(a A) func(B) R { return func(b B) R { return f(a, b) } }
}

func Each[T any](xs []T, f func(int, T)) {
	for i, x := range xs {
		f(i, x)
	}
}

func Funcs[T Num](xs ...T) []func() T {
	var out []func() T
	for _, x := range xs {
		out = append(out, func() T { x *= 2; return x })
	}
	return out
}

func main() {
	for _, c := range Counters[int8](3) {
		fmt.Print(c(), c(), c(), ";")
	}
	fmt.Println()
	add, get := Accum(100.5)
	add(1)
	add(2)
	fmt.Println(get(), add(-3.5))
	a8, g8 := Accum[int8](120)
	a8(10)
	fmt.Println(g8(), a8(10))
	fib, calls := Memo(func(n int) string { return fmt.Sprint("f", n*n) })
	fmt.Println(fib(3), fib(3), fib(4), fib(3), *calls)
	fact := Fix(func(rec func(int) int, n int) int {
		if n <= 1 {
			return 1
		}
		return n * rec(n-1)
	})
	fmt.Println(fact(5), fact(10))
	fact8 := Fix(func(rec func(int8) int8, n int8) int8 {
		if n <= 1 {
			return 1
		}
		return n * rec(n-1)
	})
	fmt.Println(fact8(5), fact8(6), fact8(7), fact8(8))
	toStr := Compose(func(i int) float64 { return float64(i) / 4 }, func(f float64) string { return fmt.Sprintf("%.2f", f) })
	fmt.Println(toStr(1), toStr(7))
	addc := Curry(func(a int8, b uint8) int { return int(a) + int(b) })
	fmt.Println(addc(-1)(255), addc(127)(1))
	var out []string
	Each([]string{"a", "b", "c"}, func(i int, s string) { out = append(out, fmt.Sprint(i, s)) })
	fmt.Println(out)
	fs := Funcs[int8](40, 60, 100)
	for _, f := range fs {
		fmt.Print(f(), f(), ";")
	}
	fmt.Println()
	var hooks []func()
	for _, x := range []float64{0.5, 1.5} {
		hooks = append(hooks, func() { x *= 2; fmt.Print(x, ";") })
	}
	for _, h := range hooks {
		h()
		h()
	}
	fmt.Println()
}

// Output:
// 1 2 3;2 4 6;3 6 9;
// 103.5 100
// -126 -116
// f9 f9 f16 f9 2
// 120 3628800
// 120 -48 -80 -128
// 0.25 1.75
// 254 128
// [0a 1b 2c]
// 80 -96;120 -16;-56 -112;
// 1;2;3;6;
