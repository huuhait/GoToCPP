// phase: 07

// Comparison with nil and conversions through core types: slices, maps, pointers, funcs, struct conversions.
package main

import "fmt"

type Ints []int
type Dict map[string]int
type Handler func() int
type IntPtr *int

type A struct {
	X int
	Y string
}
type B struct {
	X int
	Y string
}

func IsNilS[S ~[]E, E any](s S) bool           { return s == nil }
func IsNilM[M ~map[K]V, K comparable, V any](m M) bool { return m == nil }
func IsNilP[P ~*E, E any](p P) bool            { return p == nil }
func IsNilF[F ~func() R, R any](f F) bool      { return f == nil }
func Deref[P ~*E, E any](p P, def E) E {
	if p != nil {
		return *p
	}
	return def
}
func ToSlice[S ~[]E, E any](a [3]E) S          { return S(a[:]) }
func Retype[To, From ~[]E, E any](f From) To   { return To(f) }
func Struct[To, From ~struct {
	X int
	Y string
}](f From) To {
	return To(f)
}
func Text[T ~[]byte | ~[]rune](t T) string      { return string(t) }
func Fn[F ~func() int](f F) func() int          { return f }
func Flat[S ~[]E, E ~[]R, R any](ss S) (out []R) {
	for _, s := range ss {
		out = append(out, s...)
	}
	return
}

func main() {
	var ns Ints
	var nd Dict
	var np *int
	var nf Handler
	n := 5
	fmt.Println(IsNilS(ns), IsNilS(Ints{}), IsNilS([]string(nil)), IsNilS(Ints{}[:0]), IsNilM(nd), IsNilM(Dict{}), IsNilP(np), IsNilP(&n), IsNilP(IntPtr(nil)), IsNilF(nf), IsNilF(Handler(func() int { return 1 })))
	fmt.Println(Deref(np, -1), Deref(&n, -1), Deref(IntPtr(&n), 0))
	fmt.Println(ToSlice[Ints]([3]int{1, 2, 3}), ToSlice[[]string]([3]string{"a", "b", "c"}), Retype[[]int](Ints{4, 5}), Retype[Ints]([]int{6}))
	fmt.Println(Struct[B](A{1, "a"}), Struct[A](B{2, "b"}), Struct[B](struct {
		X int
		Y string
	}{3, "c"}))
	fmt.Println(Text([]byte("héllo")), Text([]rune("héllo")))
}

// Output:
// true false true false true false true false true true false
// -1 5 5
// [1 2 3] [a b c] [4 5] [6]
// {1 a} {2 b} {3 c}
// héllo héllo
