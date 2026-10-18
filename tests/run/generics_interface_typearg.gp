// phase: 07

// Interface types as type arguments: method calls dispatch dynamically, ==, nil and error.
package main

import (
	"errors"
	"fmt"
)

type Shape interface{ Area() float64 }

type Rect struct{ w, h float64 }
type Circle struct{ r float64 }

func (r Rect) Area() float64    { return r.w * r.h }
func (c *Circle) Area() float64 { return 3 * c.r * c.r }

func Total[S Shape](shapes []S) float64 {
	t := 0.0
	for _, s := range shapes {
		t += s.Area()
	}
	return t
}

func First[T any](xs []T) (T, bool) {
	if len(xs) == 0 {
		var zero T
		return zero, false
	}
	return xs[0], true
}

func IsNil[T comparable](v T) bool {
	var zero T
	return v == zero
}

func Errs[E error](es []E) string {
	out := ""
	for _, e := range es {
		out += e.Error() + ";"
	}
	return out
}

type MyErr struct{ code int }

func (e MyErr) Error() string { return fmt.Sprint("code ", e.code) }

func main() {
	fmt.Println(Total([]Shape{Rect{2, 3}, &Circle{1}}), Total([]Rect{{1, 1}, {2, 2}}), Total([]*Circle{{2}}))
	s, ok := First([]Shape{Rect{1, 2}})
	fmt.Println(s, ok)
	s, ok = First([]Shape(nil))
	fmt.Println(s, ok, s == nil)
	fmt.Println(IsNil[Shape](nil), IsNil[Shape](Rect{}), IsNil[error](nil), IsNil[error](errors.New("x")), IsNil[any](nil), IsNil[any](0))
	fmt.Println(IsNil[*int](nil), IsNil(new(int)), IsNil(0), IsNil(""), IsNil(Rect{}), IsNil(Rect{1, 0}))
	fmt.Println(Errs([]error{errors.New("a"), MyErr{2}}), Errs([]MyErr{{1}, {3}}))
	var e error = MyErr{9}
	fmt.Println(Errs([]error{e, nil}[:1]))
}

// Output:
// 9 5 12
// {1 2} true
// <nil> false true
// true false true false true false
// true false true true true false
// a;code 2; code 1;code 3;
// code 9;
