// phase: 05

// Generic functions and methods with C locals, instantiated with C and Go types: the analysis of an
// instance is its own (a generic function's type information changes for each instance), in a function
// literal, a range-over-func body with a defer and a declaration with a defer (ADR-139).
package main

import (
	"fmt"
	"iter"
)

import "./cvar_addr_generics.hpp"

var gcur *C.int
var gpair *cag.Pair

func clobber() int {
	var a [64]int
	for i := range a {
		a[i] = i + 1000
	}
	return a[5]
}

func gKeep[T any](v T) *T {
	var x T = v
	p := &x
	return p
}

type HC struct{ p *C.int }
type HI struct{ p *int }

func gStore[T any](v T, set func(*T)) {
	var x T = v
	set(&x)
}

func gAny[T any](v T) any {
	var x T = v
	return &x
}

func gSlice[T any](v T) []*T {
	var out []*T
	for i := 0; i < 2; i++ {
		var x T = v
		out = append(out, &x)
	}
	return out
}

func gClos[T any](v T) *T {
	var cur *T
	func() {
		var y T = v
		cur = &y
	}()
	return cur
}

func gHold[T any](v T) *T {
	var x T = v
	var cur *T
	for i := 0; i < 1; i++ {
		cur = &x
	}
	return cur
}

func s1() int {
	p := gKeep[C.int](7)
	clobber()
	return int(*p)
}

func s2() int {
	q := gKeep[int](9)
	p := gKeep[C.int](7)
	clobber()
	return int(*p) + *q
}

func s3() int {
	p := gKeep[C.int](7)
	q := gKeep[int](9)
	clobber()
	return int(*p) + *q
}

func s4() int {
	var d HC
	gStore[C.int](11, func(p *C.int) { d.p = p })
	var e HI
	gStore[int](13, func(p *int) { e.p = p })
	clobber()
	return int(*d.p) + *e.p
}

func s5() int {
	b := gAny[int](6)
	clobber()
	return *(b.(*int))
}

func s6() int {
	a := gSlice[C.int](3)
	b := gSlice[int](4)
	clobber()
	return int(*a[0]) + int(*a[1]) + *b[0] + *b[1]
}

func s7() int {
	a := gClos[C.int](21)
	b := gClos[int](22)
	clobber()
	return int(*a) + *b
}

func s8() int {
	a := gHold[C.int](31)
	b := gHold[int](32)
	clobber()
	return int(*a) + *b
}

func s9() int {
	p := gKeep[cag.Pair](cag.Pair{a: 4, b: 5})
	q := gKeep[struct{ a, b int }](struct{ a, b int }{6, 7})
	clobber()
	return int(p.a+p.b) + q.a + q.b
}


func cleanup() {}

func seq(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; i < n; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

func gLoc[T any](v T) *C.int {
	var x C.int = C.int(3)
	var cur *C.int
	func() {
		cur = &x
	}()
	return cur
}

func gLocClos[T any](v T) *C.int {
	var cur *C.int
	func() {
		var y C.int = C.int(4)
		cur = &y
	}()
	return cur
}

type GBox[T any] struct{ v T }

func (b *GBox[T]) Ptr() *C.int {
	var x C.int = 9
	p := &x
	return p
}

func (b *GBox[T]) Cl() *C.int {
	var cur *C.int
	func() {
		var y C.int = 8
		cur = &y
	}()
	return cur
}

func gRange[T any](v T) *C.int {
	defer cleanup()
	var cur *C.int
	for i := range seq(2) {
		var x C.int = C.int(10 + i)
		cur = &x
	}
	return cur
}

func gDecl[T any](v T) *C.int {
	defer cleanup()
	var x C.int = 6
	cur := &x
	return cur
}

func iv(p *C.int) int {
	return int(*p)
}

func s10() int {
	a := gLoc[int](1)
	b := gLoc[string]("a")
	c := gLoc[float64](2)
	clobber()
	return iv(a) + iv(b) + iv(c)
}

func s11() int {
	a := gLocClos[int](1)
	b := gLocClos[string]("a")
	c := gLocClos[float64](2)
	clobber()
	return iv(a) + iv(b) + iv(c)
}

func s12() int {
	a := (&GBox[int]{}).Ptr()
	b := (&GBox[string]{}).Ptr()
	c := (&GBox[int]{}).Cl()
	d := (&GBox[string]{}).Cl()
	clobber()
	return iv(a) + iv(b) + iv(c) + iv(d)
}

func s13() int {
	a := gRange[int](1)
	b := gRange[string]("a")
	clobber()
	return iv(a) + iv(b)
}

func s14() int {
	a := gDecl[int](1)
	b := gDecl[string]("a")
	clobber()
	return iv(a) + iv(b)
}

func main() {
	fmt.Println(s1(), s2(), s3(), s4(), s5(), s6(), s7())
	fmt.Println(s8(), s9(), s10(), s11(), s12(), s13(), s14())
}

// Output:
// 7 16 16 24 6 14 43
// 63 22 9 12 34 22 12
