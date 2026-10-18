// phase: 07

// Func types as type arguments: calling values of type T where T is a func type, and funcs in maps and slices.
package main

import "fmt"

type Handler func(string) string
type Pred func(int) bool

func Call[F ~func(string) string](f F, s string) string { return f(s) }

func Apply[T any, F ~func(T) T](fs []F, v T) T {
	for _, f := range fs {
		v = f(v)
	}
	return v
}

func Wrap[F ~func(int) bool](f F) F {
	return func(i int) bool { return !f(i) }
}

func Run[T any](fs ...func() T) []T {
	var out []T
	for _, f := range fs {
		out = append(out, f())
	}
	return out
}

func Table[K comparable, F any](keys []K, f func(K) F) map[K]F {
	m := map[K]F{}
	for _, k := range keys {
		m[k] = f(k)
	}
	return m
}

func Id[T any](v T) T { return v }

func main() {
	up := Handler(func(s string) string { return s + "!" })
	fmt.Println(Call(up, "hi"), Call(func(s string) string { return s + s }, "ab"))
	fmt.Println(Apply([]Handler{up, up}, "x"), Apply([]func(int) int{func(i int) int { return i + 1 }, func(i int) int { return i * 3 }}, 4))
	even := Pred(func(i int) bool { return i%2 == 0 })
	odd := Wrap(even)
	fmt.Println(even(2), odd(2), odd(3))
	fmt.Printf("%T\n", odd)
	fmt.Println(Run(func() int { return 1 }, func() int { return 2 }), Run[string]())
	tbl := Table([]int{1, 2, 3}, func(i int) func() int { return func() int { return i * i } })
	fmt.Println(tbl[1](), tbl[2](), tbl[3](), len(tbl))
	fs := Id([]func(int) int{func(i int) int { return -i }})
	fmt.Println(fs[0](5), Id(func() string { return "x" })(), Id(up)("z"))
}

// Output:
// hi! abab
// x!! 15
// true false true
// main.Pred
// [1 2] []
// 1 4 9 3
// -5 x z!
