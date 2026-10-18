// phase: 07

// x.(T) with a type parameter T: comma-ok, and the panic text when it fails.
package main

import "fmt"

type Named interface{ Name() string }
type Reader interface{ Read(p []byte) int }
type Dog struct{ n string }

func (d Dog) Name() string { return d.n }

func As[T any](v any) (T, bool) {
	t, ok := v.(T)
	return t, ok
}

func Must[T any](v any) T { return v.(T) }

func Or[T any](v any, def T) T {
	if t, ok := v.(T); ok {
		return t
	}
	return def
}

func Names[T Named](vs []any) []string {
	var out []string
	for _, v := range vs {
		if t, ok := v.(T); ok {
			out = append(out, t.Name())
		}
	}
	return out
}

func main() {
	fmt.Println(As[int](5))
	fmt.Println(As[string](5))
	fmt.Println(As[Named](Dog{"rex"}))
	fmt.Println(As[Named](5))
	fmt.Println(As[any](nil))
	fmt.Println(As[error](nil))
	fmt.Println(As[Reader](Dog{"x"}))
	fmt.Println(Or(5, 0), Or("x", 0), Or[any](nil, 7), Or([]int{1}, []int(nil)))
	fmt.Println(Names[Dog]([]any{Dog{"a"}, 1, Dog{"b"}}), Names[Named]([]any{Dog{"c"}, "s"}))
	fmt.Println(Must[int](3), Must[Named](Dog{"z"}).Name())
	defer func() { fmt.Println(recover()) }()
	fmt.Println(Must[string](3))
}

// Output:
// 5 true
//  false
// {rex} true
// <nil> false
// <nil> false
// <nil> false
// <nil> false
// 5 0 7 [1]
// [a b] [c]
// 3 z
// interface conversion: interface {} is int, not string
