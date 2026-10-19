// phase: 07

// Panics, defers and recover in generic code: panic values of type-parameter type, re-panics,
// errors.As into an instance of a generic error type, deferred calls of generic methods in LIFO
// order, recover returning runtime errors with Go's texts, and a panic crossing instances.
package main

import (
	"errors"
	"fmt"
)

type Err[T any] struct {
	Op  string
	Val T
}

func (e *Err[T]) Error() string { return fmt.Sprintf("%s failed on %v", e.Op, e.Val) }

type Wrap[T any] struct {
	Inner error
	Ctx   T
}

func (w Wrap[T]) Error() string { return fmt.Sprint("wrap(", w.Ctx, "): ", w.Inner) }
func (w Wrap[T]) Unwrap() error { return w.Inner }

type Guard[T any] struct{ log *[]string }

func (g Guard[T]) Leave(tag string) { *g.log = append(*g.log, "leave "+tag) }
func (g *Guard[T]) Enter(tag T) func() {
	*g.log = append(*g.log, fmt.Sprint("enter ", tag))
	return func() { *g.log = append(*g.log, fmt.Sprint("exit ", tag)) }
}

func Try[T any](f func() T) (res T, err error) {
	defer func() {
		switch r := recover().(type) {
		case nil:
		case error:
			err = r
		default:
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return f(), nil
}

func Throw[T any](v T) T { panic(v) }

func Chain[T any](depth int, v T) T {
	if depth == 0 {
		panic(&Err[T]{"chain", v})
	}
	defer func() {
		if r := recover(); r != nil {
			panic(Wrap[int]{r.(error), depth})
		}
	}()
	return Chain(depth-1, v)
}

func Index[S ~[]E, E any](s S, i int) E { return s[i] }

func Div[T ~int | ~int8](a, b T) T { return a / b }

func main() {
	fmt.Println(Try(func() int { return Throw(42) }))
	fmt.Println(Try(func() string { return Throw("boom") }))
	fmt.Println(Try(func() []int { return Throw[[]int](nil) }))
	fmt.Println(Try(func() float64 { return Throw(1.5) }))
	_, err := Try(func() error { return Throw(errors.New("an error")) })
	fmt.Println(err)
	_, err = Try(func() int { return Index([]int{1, 2}, 5) })
	fmt.Println(err)
	_, err = Try(func() string { return Index([]string(nil), 0) })
	fmt.Println(err)
	_, err = Try(func() int8 { return Div(int8(1), 0) })
	fmt.Println(err)
	_, err = Try(func() int { var m map[string]int; m["a"] = 1; return 0 })
	fmt.Println(err)
	_, err = Try(func() *Err[int] { var e *Err[int]; return &Err[int]{e.Op, 1} })
	fmt.Println(err)
	_, err = Try(func() int { var a any = "s"; return a.(int) })
	fmt.Println(err)
	_, err = Try(func() Err[int] { return Chain(3, Err[int]{"inner", 1}) })
	fmt.Println(err)
	_, err = Try(func() int { return Chain(3, 7) })
	fmt.Println(err)
	var w Wrap[int]
	var e *Err[int]
	fmt.Println(errors.As(err, &w), w.Ctx, errors.As(err, &e), e.Op, e.Val)
	var e2 *Err[string]
	fmt.Println(errors.As(err, &e2), errors.Is(err, w.Inner), errors.Unwrap(err) == w.Inner)
	var log []string
	g := &Guard[int]{&log}
	func() {
		defer g.Leave("a")
		defer g.Enter(1)()
		defer func() { recover() }()
		defer g.Leave("b")
		g.Enter(2)
		panic("x")
	}()
	fmt.Println(log)
	fmt.Println(Try(func() int {
		defer func() { panic("second") }()
		panic("first")
	}))
}

// Output:
// 0 panic: 42
//  panic: boom
// [] panic: []
// 0 panic: 1.5
// an error
// runtime error: index out of range [5] with length 2
// runtime error: index out of range [0] with length 0
// runtime error: integer divide by zero
// assignment to entry in nil map
// runtime error: invalid memory address or nil pointer dereference
// interface conversion: interface {} is string, not int
// wrap(3): wrap(2): wrap(1): chain failed on {inner 1}
// wrap(3): wrap(2): wrap(1): chain failed on 7
// true 3 true chain 7
// false true true
// [enter 1 enter 2 leave b exit 1 leave a]
// 0 panic: second
