// phase: 07

// Option and Result style generic types with methods that return other instances.
package main

import (
	"errors"
	"fmt"
)

type Opt[T any] struct {
	v  T
	ok bool
}

func Some[T any](v T) Opt[T] { return Opt[T]{v, true} }
func None[T any]() Opt[T]    { return Opt[T]{} }

func (o Opt[T]) IsSome() bool { return o.ok }

func (o Opt[T]) OrElse(d T) T {
	if o.ok {
		return o.v
	}
	return d
}

func (o Opt[T]) Filter(p func(T) bool) Opt[T] {
	if o.ok && p(o.v) {
		return o
	}
	return None[T]()
}

func (o Opt[T]) String() string {
	if !o.ok {
		return "None"
	}
	return fmt.Sprintf("Some(%v)", o.v)
}

func MapOpt[T, U any](o Opt[T], f func(T) U) Opt[U] {
	if !o.ok {
		return None[U]()
	}
	return Some(f(o.v))
}

type Result[T any] struct {
	val T
	err error
}

func Ok[T any](v T) Result[T]        { return Result[T]{val: v} }
func Fail[T any](e error) Result[T]  { return Result[T]{err: e} }

func (r Result[T]) Unwrap() T {
	if r.err != nil {
		panic(r.err)
	}
	return r.val
}

func (r Result[T]) Err() error { return r.err }

func Parse(s string) Result[int] {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return Fail[int](errors.New("bad digit in " + s))
		}
		n = n*10 + int(c-'0')
	}
	return Ok(n)
}

func main() {
	a := Some(4)
	fmt.Println(a, None[string](), a.Filter(func(i int) bool { return i > 5 }), a.Filter(func(i int) bool { return i > 3 }))
	fmt.Println(MapOpt(a, func(i int) string { return fmt.Sprint(i * 2) }), MapOpt(None[int](), func(i int) int { return i }))
	fmt.Println(a.OrElse(0), None[float64]().OrElse(1.5), a.IsSome())
	fmt.Printf("%v %T\n", Some("x"), MapOpt(a, func(i int) []int { return nil }))
	fmt.Println(Parse("123").Unwrap(), Parse("12x").Err())
	defer func() { fmt.Println("recovered:", recover()) }()
	Parse("zz").Unwrap()
}

// Output:
// Some(4) None None Some(4)
// Some(8) None
// 4 1.5 true
// Some(x) main.Opt[[]int]
// 123 bad digit in 12x
// recovered: bad digit in zz
