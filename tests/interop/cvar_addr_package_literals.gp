// phase: 05

// Function literals in package-level initializers (and in init, methods and method values): a literal in
// the second, third ... initializer is analysed with its own initializer, not the first one. Each local
// whose address leaves (a returned pointer, a store into a package variable, a nested literal, a
// range-over-func body with a defer, a goroutine) is boxed (ADR-139).
package main

import (
	"fmt"
	"iter"
	"sync"
)

import "./cvar_addr_package_literals.hpp"

var gcur *C.int
var gcur2 *C.int

func clobber() int {
	var a [64]int
	for i := range a {
		a[i] = i + 1000
	}
	return a[5]
}

func cleanup() {}

func iv(p *C.int) int {
	return int(*p)
}

func seq(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; i < n; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

type Fns struct {
	get func() *C.int
	n   int
}

var pf func() *C.int = func() *C.int {
	var y C.int = 41
	p := &y
	return p
}

var pb = Fns{get: func() *C.int {
	var y C.int = 42
	p := &y
	return p
}, n: 1}

var pn func() *C.int = func() *C.int {
	var cur *C.int
	func() {
		var y C.int = 43
		cur = &y
	}()
	return cur
}

var pr func() *C.int = func() *C.int {
	defer cleanup()
	var cur *C.int
	for i := range seq(2) {
		var x C.int = C.int(44 + i)
		cur = &x
	}
	return cur
}

var pcopy func() *C.int = func() *C.int {
	var cur *C.int
	for i := range seq(2) {
		var x C.int = C.int(60 + i)
		cur = &x
	}
	return cur
}

var pstore func() int = func() int {
	var x C.int = 70
	gcur2 = &x
	return 0
}

func init() {
	var x C.int = 46
	gcur = &x
}

type B struct{ n int }

func (b *B) Get() *C.int {
	var x C.int = C.int(47 + b.n)
	p := &x
	return p
}

func (b B) GetV() *C.int {
	var x C.int = C.int(49 + b.n)
	p := &x
	return p
}

func (b *B) Closure() *C.int {
	defer cleanup()
	var cur *C.int
	func() {
		var y C.int = C.int(51 + b.n)
		cur = &y
	}()
	return cur
}

func q1() int {
	p := pf()
	clobber()
	return iv(p)
}

func q2() int {
	p := pb.get()
	clobber()
	return iv(p)
}

func q3() int {
	p := pn()
	clobber()
	return iv(p)
}

func q4() int {
	p := pr()
	clobber()
	return iv(p)
}

func q5() int {
	clobber()
	return iv(gcur)
}

func q6() int {
	b := &B{n: 0}
	f := b.Get
	p := f()
	clobber()
	return iv(p)
}

func q7() int {
	b := B{n: 0}
	f := b.GetV
	p := f()
	clobber()
	return iv(p)
}

func q8() int {
	b := &B{n: 0}
	p := b.Closure()
	clobber()
	return iv(p)
}

func q9() int {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		var y C.int = 53
		gcur = &y
	}()
	wg.Wait()
	clobber()
	return iv(gcur)
}

func q10() int {
	var wg sync.WaitGroup
	wg.Add(1)
	var x C.int = 54
	go func(p *C.int) {
		defer wg.Done()
		gcur2 = p
	}(&x)
	wg.Wait()
	return iv(gcur2)
}

func q11() int {
	func() {
		defer func() {
			var y C.int = 55
			gcur = &y
		}()
	}()
	clobber()
	return iv(gcur)
}

func q12() int {
	var cur *C.int
outer:
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			func() {
				var y C.int = C.int(56 + i)
				cur = &y
			}()
			if j == 1 {
				continue outer
			}
		}
	}
	clobber()
	return iv(cur)
}

func callIt(f func()) { f() }

func q13() int {
	var cur *C.int
	f := func() {
		var x C.int = 57
		g := func() { cur = &x }
		g()
	}
	callIt(f)
	clobber()
	return iv(cur)
}

func q14() int {
	var x C.int = 58
	var cur *C.int
	callIt(func() { cur = &x })
	clobber()
	return iv(cur)
}

func q15() int {
	defer cleanup()
	var cur *C.int
	callIt(func() {
		defer cleanup()
		var y C.int = 59
		cur = &y
	})
	clobber()
	return iv(cur)
}

func q16() int {
	pstore()
	clobber()
	return iv(gcur2)
}

func main() {
	fmt.Println(q1(), q2(), q3(), q4(), q5(), q6())
	fmt.Println(q7(), q8(), q9(), q10(), q11(), q12())
	fmt.Println(q13(), q14(), q15(), q16(), iv(pcopy()))
}

// Output:
// 41 42 43 45 46 47
// 49 51 53 54 55 58
// 57 58 59 70 61
