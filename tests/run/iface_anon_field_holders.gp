// phase: 05

// Unnamed interface types held by value in struct fields (a named struct, an array, a pointer, an
// unnamed struct, a function type, a generic instance) that no function assigns into: the C++ struct
// of the interface must be defined before the struct that holds it.
package main

import "fmt"

type S struct{ n int }

func (s S) Get() int { return s.n }
func (s S) Name() string { return "S" }

// value-type holders of an anonymous interface, never assigned anywhere
type ArrHolder struct{ a [2]interface{ Get() int } }
type PtrHolder struct{ p *interface{ Get() int } }
type NestedAnon struct {
	in struct{ v interface{ Get() int } }
}
type Elems []interface{ Get() int }
type Dict map[string]interface{ Get() int }
type Fn func(interface{ Get() int }) interface{ Name() string }
type Alias = interface{ Get() int }
type AliasHolder struct{ v Alias }
type Two struct {
	a interface{ Get() int }
	b interface {
		Name() string
		Get() int
	}
	c interface{ Name() string }
}

// a named interface whose method mentions anonymous interfaces
type Visitor interface {
	Visit(interface{ Get() int }) interface{ Name() string }
}

type Gen[T any] struct {
	v T
	w interface{ Get() int }
}

type GenIface[T interface{ Get() int }] struct{ items []T }

type Outer struct {
	h  ArrHolder
	pp PtrHolder
}

func main() {
	var a ArrHolder
	var p PtrHolder
	var n NestedAnon
	var e Elems
	var d Dict
	var f Fn
	var ah AliasHolder
	var t Two
	var v Visitor
	var g Gen[int]
	var gi Gen[interface{ Get() int }]
	var gs GenIface[S]
	var o Outer
	fmt.Println(a.a[0] == nil, a.a[1] == nil, p.p == nil, n.in.v == nil, e == nil, d == nil, f == nil)
	fmt.Println(ah.v == nil, t.a == nil, t.b == nil, t.c == nil, v == nil, g.w == nil, gi.v == nil, len(gs.items))
	fmt.Println(o.h.a[0] == nil, o.pp.p == nil)
	a.a[1] = S{4}
	fmt.Println(a.a[1].Get())
	var x interface{ Get() int } = S{5}
	p.p = &x
	fmt.Println((*p.p).Get())
	t.b = S{6}
	fmt.Println(t.b.Name(), t.b.Get())
	fmt.Println(a == a, t == t)
}

// Output:
// true true true true true true true
// true true true true true true true 0
// true true
// 4
// 5
// S 6
// true true
