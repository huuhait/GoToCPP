// phase: 07

// Instantiated generic functions as values: stored, passed, returned and compared with nil.
package main

import "fmt"

func Id[T any](x T) T { return x }

func Twice[T any](f func(T) T) func(T) T {
	return func(x T) T { return f(f(x)) }
}

func Apply[T, U any](f func(T) U, x T) U { return f(x) }

func Map[T, U any](xs []T, f func(T) U) []U {
	var out []U
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func Neg[T ~int | ~float64](x T) T { return -x }

func main() {
	f := Id[int]
	fmt.Println(f(3), f != nil)
	var g func(string) string = Id
	fmt.Println(g("s"))
	h := Map[int, string]
	fmt.Println(h([]int{1, 2}, func(i int) string { return fmt.Sprint(i, i) }))
	fmt.Println(Apply(Neg[int], 5), Apply(Neg[float64], 2.5), Apply(Id[bool], true))
	fmt.Println(Twice(Neg[int])(7), Twice(Id[string])("x"))
	fs := []func(float64) float64{Neg[float64], Id[float64], Twice(Neg[float64])}
	fmt.Println(fs[0](1.5), fs[1](1.5), fs[2](1.5))
	m := map[string]func(int) int{"neg": Neg[int], "id": Id[int]}
	fmt.Println(m["neg"](4), m["id"](4))
	fmt.Println(Map([]int{1, 2}, Neg[int]))
}

// Output:
// 3 true
// s
// [1 1 2 2]
// -5 -2.5 true
// 7 x
// -1.5 1.5 1.5
// -4 4
// [-1 -2]
