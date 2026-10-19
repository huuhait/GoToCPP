// phase: 07
// lsan: cycle

// Closures in generic functions: captured type parameters, function-typed results, recursion, loops.
package main

import "fmt"

type Num interface{ ~int | ~int8 | ~float64 }

func Counter[T Num](start, step T) func() T {
	n := start
	return func() T {
		n += step
		return n
	}
}

func Compose[A, B, R any](f func(A) B, g func(B) R) func(A) R {
	return func(a A) R { return g(f(a)) }
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

func Fix[T any](f func(func(T) T, T) T) func(T) T {
	var rec func(T) T
	rec = func(x T) T { return f(rec, x) }
	return rec
}

func Each[T any](xs []T, f func(int, T)) {
	for i, x := range xs {
		f(i, x)
	}
}

func Collect[T any](xs []T) (fs []func() T) {
	for _, x := range xs {
		fs = append(fs, func() T { return x })
	}
	return
}

func Adder[T Num](xs ...T) func(T) T {
	return func(y T) T {
		for _, x := range xs {
			y += x
		}
		return y
	}
}

func Partial[A, B, R any](f func(A, B) R, a A) func(B) R {
	return func(b B) R { return f(a, b) }
}

func Once[T any](f func() T) func() T {
	var done bool
	var v T
	return func() T {
		if !done {
			v, done = f(), true
		}
		return v
	}
}

func main() {
	c := Counter(10, 5)
	d := Counter(int8(120), 5)
	e := Counter(0.5, 0.25)
	fmt.Println(c(), c(), d(), d(), e(), e(), c())
	f := Compose(func(i int) string { return fmt.Sprint("<", i, ">") }, func(s string) int { return len(s) })
	fmt.Println(f(5), f(12345), Compose(func(s string) []byte { return []byte(s) }, func(b []byte) int { return len(b) })("héllo"))
	sq, calls := Memo(func(n int) int { return n * n })
	fmt.Println(sq(4), sq(4), sq(5), *calls)
	fact := Fix(func(rec func(int) int, n int) int {
		if n <= 1 {
			return 1
		}
		return n * rec(n-1)
	})
	fmt.Println(fact(10), Fix(func(rec func(string) string, s string) string {
		if len(s) <= 1 {
			return s
		}
		return rec(s[1:]) + s[:1]
	})("abcd"))
	total := 0
	Each([]int{5, 6, 7}, func(i, x int) { total += i * x })
	fs := Collect([]string{"a", "b", "c"})
	fmt.Println(total, fs[0](), fs[1](), fs[2](), Adder(1, 2, 3)(10), Adder(0.5)(0.25), Adder[int8]()(7))
	p := Partial(func(a string, b int) string { return fmt.Sprint(a, b) }, "x")
	n := 0
	o := Once(func() int { n++; return n * 10 })
	fmt.Println(p(1), p(22), o(), o(), n)
}

// Output:
// 15 20 125 -126 0.75 1 25
// 3 7 6
// 16 16 25 2
// 3628800 dcba
// 20 a b c 16 0.75 7
// x1 x22 10 10 1
