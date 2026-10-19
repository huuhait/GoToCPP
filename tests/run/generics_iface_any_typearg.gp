// phase: 07

// Interface types as type arguments: any, error, fmt.Stringer and custom interfaces, with nil and dynamic types.
package main

import (
	"errors"
	"fmt"
)

type Shape interface{ Area() int }
type Rect struct{ W, H int }
type Circle struct{ R int }

func (r Rect) Area() int    { return r.W * r.H }
func (c *Circle) Area() int { return 3 * c.R * c.R }

func Largest[T Shape](xs ...T) (best T) {
	for i, x := range xs {
		if i == 0 || x.Area() > best.Area() {
			best = x
		}
	}
	return
}

func First[T any](xs []T) (T, bool) {
	if len(xs) == 0 {
		var z T
		return z, false
	}
	return xs[0], true
}

func Ptr[T any](v T) *T { return &v }

func Filter[T any](xs []T, keep func(T) bool) []T {
	var out []T
	for _, x := range xs {
		if keep(x) {
			out = append(out, x)
		}
	}
	return out
}

func Show[T any](xs ...T) string { return fmt.Sprint(xs) }

func main() {
	shapes := []Shape{Rect{2, 3}, &Circle{2}, Rect{1, 1}}
	fmt.Println(Largest(shapes...).Area(), Largest(Rect{1, 2}, Rect{3, 4}), Largest[Shape](), Largest[Shape]() == nil)
	v, ok := First([]error{errors.New("e1"), nil})
	w, ok2 := First([]error{})
	fmt.Println(v, ok, w, ok2, w == nil)
	a, _ := First([]any{nil, 1})
	b, _ := First([]any{1, nil})
	fmt.Println(a, b, a == nil, b == nil)
	p := Ptr[Shape](Rect{5, 5})
	q := Ptr[any](nil)
	r := Ptr[error](nil)
	fmt.Println((*p).Area(), *q == nil, *r == nil, *Ptr[fmt.Stringer](nil) == nil)
	errs := Filter([]error{errors.New("a"), nil, errors.New("b")}, func(e error) bool { return e != nil })
	vals := Filter([]any{1, "x", nil, 2.5, []int{}}, func(x any) bool { return x != nil })
	fmt.Println(errs, len(errs), vals, len(vals))
	fmt.Println(Show[any](1, "a", nil, Rect{1, 2}), Show[error](nil, errors.New("x")), Show[Shape](Rect{1, 2}, &Circle{1}) != "")
	fmt.Printf("%T %T %T\n", Largest[Shape], First[error], Ptr[any])
}

// Output:
// 12 {3 4} <nil> true
// e1 true <nil> false true
// <nil> 1 true false
// 25 true true true
// [a b] 2 [1 x 2.5 []] 4
// [1 a <nil> {1 2}] [<nil> x] true
// func(...main.Shape) main.Shape func([]error) (error, bool) func(interface {}) *interface {}
