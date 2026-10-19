// phase: 07

// Method expressions and method values of defined types used with generic functions as function values.
package main

import "fmt"

type Vec struct{ X, Y int }

func (v Vec) Add(w Vec) Vec     { return Vec{v.X + w.X, v.Y + w.Y} }
func (v Vec) String() string    { return fmt.Sprintf("<%d,%d>", v.X, v.Y) }
func (v *Vec) Scale(k int)      { v.X *= k; v.Y *= k }

func Fold[T, A any](xs []T, init A, f func(A, T) A) A {
	acc := init
	for _, x := range xs {
		acc = f(acc, x)
	}
	return acc
}

func Map[T, R any](xs []T, f func(T) R) []R {
	out := make([]R, 0, len(xs))
	for _, x := range xs {
		out = append(out, f(x))
	}
	return out
}

func Apply[T any](x T, fs ...func(T) T) T {
	for _, f := range fs {
		x = f(x)
	}
	return x
}

func Each[T any](xs []T, f func(*T)) {
	for i := range xs {
		f(&xs[i])
	}
}

func Id[T any](x T) T { return x }

func main() {
	vs := []Vec{{1, 2}, {3, 4}, {5, 6}}
	fmt.Println(Fold(vs, Vec{}, Vec.Add), Map(vs, Vec.String), Map(vs, Id[Vec]))
	unit := Vec{1, 1}
	fmt.Println(Fold(vs, unit, func(a Vec, v Vec) Vec { return a.Add(v) }), Apply(unit, unit.Add, unit.Add, Id))
	Each(vs, func(p *Vec) { p.Scale(10) })
	scale3 := (*Vec).Scale
	Each(vs, func(p *Vec) { scale3(p, 3) })
	fmt.Println(vs)
	f := Map[int, string]
	g := Id[[]int]
	h := Fold[int, int]
	fmt.Println(f([]int{1, 2}, func(i int) string { return fmt.Sprint("n", i) }), g([]int{7}), h([]int{1, 2, 3}, 0, func(a, b int) int { return a + b*b }))
	fmt.Printf("%T %T %T\n", f, g, h)
	var fs []func(int) int
	fs = append(fs, Id[int], func(x int) int { return x + 1 })
	fmt.Println(Apply(5, fs...), Apply("s"), len(fs))
}

// Output:
// <9,12> [<1,2> <3,4> <5,6>] [<1,2> <3,4> <5,6>]
// <10,13> <3,3>
// [<30,60> <90,120> <150,180>]
// [n1 n2] [7] 14
// func([]int, func(int) string) []string func([]int) []int func([]int, int, func(int, int) int) int
// 6 s 2
