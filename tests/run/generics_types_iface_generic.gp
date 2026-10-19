// phase: 07

// Generic interface types: instances as parameter and variable types, satisfied by instances of other generic types.
package main

import "fmt"

type Getter[T any] interface{ Get() T }
type Setter[T any] interface{ Set(T) }
type GetSetter[T any] interface {
	Getter[T]
	Setter[T]
}

type Cell[T any] struct{ v T }

func (c *Cell[T]) Get() T  { return c.v }
func (c *Cell[T]) Set(v T) { c.v = v }

type Const[T any] struct{ v T }

func (c Const[T]) Get() T { return c.v }

func Read[T any](g Getter[T]) T { return g.Get() }

func Swap[T any](a, b GetSetter[T]) {
	x, y := a.Get(), b.Get()
	a.Set(y)
	b.Set(x)
}

func Sum[T ~int | ~float64](gs ...Getter[T]) T {
	var s T
	for _, g := range gs {
		s += g.Get()
	}
	return s
}

func main() {
	a, b := &Cell[int]{1}, &Cell[int]{2}
	Swap[int](a, b)
	fmt.Println(a.v, b.v, Read[int](a), Read[int](Const[int]{7}))
	fmt.Println(Sum[int](a, b, Const[int]{10}), Sum[float64](Const[float64]{0.5}, &Cell[float64]{1}))
	var g Getter[string] = Const[string]{"s"}
	var gs GetSetter[string] = &Cell[string]{"t"}
	fmt.Println(g.Get(), gs.Get())
	gs.Set("u")
	fmt.Printf("%T %T %v\n", g, gs, gs.Get())
	var nilG Getter[int]
	fmt.Println(nilG == nil, g != nil)
}

// Output:
// 2 1 2 7
// 13 1.5
// s t
// main.Const[string] *main.Cell[string] u
// true true
