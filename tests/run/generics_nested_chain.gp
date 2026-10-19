// phase: 07

// A chain of generic types that each hold the next by value (12 deep) and instances nested 8 deep:
// the layout of such a chain used to cost 3^depth in the checker (a type nested like this took minutes
// to lower), and the descriptor, %T name and methods of every level must still be right.
package main

import "fmt"

type Box0[T any] struct {
	v  T
	in Box1[T]
	w  int8
}

func (b *Box0[T]) Set(v T)              { b.v = v; b.in.Set(v) }
func (b Box0[T]) Sum(f func(T) int) int { return f(b.v) + b.in.Sum(f) + int(b.w) }

type Box1[T any] struct {
	v  T
	in Box2[T]
	w  int8
}

func (b *Box1[T]) Set(v T)              { b.v = v; b.in.Set(v) }
func (b Box1[T]) Sum(f func(T) int) int { return f(b.v) + b.in.Sum(f) + int(b.w) }

type Box2[T any] struct {
	v  T
	in Box3[T]
	w  int8
}

func (b *Box2[T]) Set(v T)              { b.v = v; b.in.Set(v) }
func (b Box2[T]) Sum(f func(T) int) int { return f(b.v) + b.in.Sum(f) + int(b.w) }

type Box3[T any] struct {
	v  T
	in Box4[T]
	w  int8
}

func (b *Box3[T]) Set(v T)              { b.v = v; b.in.Set(v) }
func (b Box3[T]) Sum(f func(T) int) int { return f(b.v) + b.in.Sum(f) + int(b.w) }

type Box4[T any] struct {
	v  T
	in Box5[T]
	w  int8
}

func (b *Box4[T]) Set(v T)              { b.v = v; b.in.Set(v) }
func (b Box4[T]) Sum(f func(T) int) int { return f(b.v) + b.in.Sum(f) + int(b.w) }

type Box5[T any] struct {
	v  T
	in Box6[T]
	w  int8
}

func (b *Box5[T]) Set(v T)              { b.v = v; b.in.Set(v) }
func (b Box5[T]) Sum(f func(T) int) int { return f(b.v) + b.in.Sum(f) + int(b.w) }

type Box6[T any] struct {
	v  T
	in Box7[T]
	w  int8
}

func (b *Box6[T]) Set(v T)              { b.v = v; b.in.Set(v) }
func (b Box6[T]) Sum(f func(T) int) int { return f(b.v) + b.in.Sum(f) + int(b.w) }

type Box7[T any] struct {
	v  T
	in Box8[T]
	w  int8
}

func (b *Box7[T]) Set(v T)              { b.v = v; b.in.Set(v) }
func (b Box7[T]) Sum(f func(T) int) int { return f(b.v) + b.in.Sum(f) + int(b.w) }

type Box8[T any] struct {
	v  T
	in Box9[T]
	w  int8
}

func (b *Box8[T]) Set(v T)              { b.v = v; b.in.Set(v) }
func (b Box8[T]) Sum(f func(T) int) int { return f(b.v) + b.in.Sum(f) + int(b.w) }

type Box9[T any] struct {
	v  T
	in Box10[T]
	w  int8
}

func (b *Box9[T]) Set(v T)              { b.v = v; b.in.Set(v) }
func (b Box9[T]) Sum(f func(T) int) int { return f(b.v) + b.in.Sum(f) + int(b.w) }

type Box10[T any] struct {
	v  T
	in Box11[T]
	w  int8
}

func (b *Box10[T]) Set(v T)              { b.v = v; b.in.Set(v) }
func (b Box10[T]) Sum(f func(T) int) int { return f(b.v) + b.in.Sum(f) + int(b.w) }

type Box11[T any] struct {
	v T
	w int8
}

func (b *Box11[T]) Set(v T)              { b.v = v }
func (b Box11[T]) Sum(f func(T) int) int { return f(b.v) + int(b.w) }

type W[T any] struct{ v T }

func (w W[T]) Get() T { return w.v }

func main() {
	var b Box0[int8]
	b.Set(100)
	fmt.Println(b.Sum(func(v int8) int { return int(v) }), b)
	var s Box0[string]
	s.Set("ab")
	fmt.Println(s.Sum(func(v string) int { return len(v) }))
	fmt.Printf("%T\n", b)
	fmt.Printf("%T\n", b.in.in.in.in.in.in.in.in.in.in.in)
	var n Box3[W[W[W[W[W[W[W[W[int]]]]]]]]]
	fmt.Printf("%T\n%+v\n", n, n)
	n.in.in.in.in.in.in.in.in.v = W[W[W[W[W[W[W[W[int]]]]]]]]{}
	fmt.Println(n.in.in.in.in.in.in.in.in.v.v.v.v.v.v.v.Get().Get())
}

// Output:
// 1200 {100 {100 {100 {100 {100 {100 {100 {100 {100 {100 {100 {100 0} 0} 0} 0} 0} 0} 0} 0} 0} 0} 0} 0}
// 24
// main.Box0[int8]
// main.Box11[int8]
// main.Box3[main.W[main.W[main.W[main.W[main.W[main.W[main.W[main.W[int]]]]]]]]]
// {v:{v:{v:{v:{v:{v:{v:{v:{v:0}}}}}}}} in:{v:{v:{v:{v:{v:{v:{v:{v:{v:0}}}}}}}} in:{v:{v:{v:{v:{v:{v:{v:{v:{v:0}}}}}}}} in:{v:{v:{v:{v:{v:{v:{v:{v:{v:0}}}}}}}} in:{v:{v:{v:{v:{v:{v:{v:{v:{v:0}}}}}}}} in:{v:{v:{v:{v:{v:{v:{v:{v:{v:0}}}}}}}} in:{v:{v:{v:{v:{v:{v:{v:{v:{v:0}}}}}}}} in:{v:{v:{v:{v:{v:{v:{v:{v:{v:0}}}}}}}} in:{v:{v:{v:{v:{v:{v:{v:{v:{v:0}}}}}}}} w:0} w:0} w:0} w:0} w:0} w:0} w:0} w:0} w:0}
// 0
