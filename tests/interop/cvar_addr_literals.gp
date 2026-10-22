// phase: 05

// A function literal stores the address of its own local (or an outer one) where the enclosing function
// reads it after clobber(): into an outer holder, three levels deep, copied into a package
// variable, from a deferred literal, in a literal that runs twice, recursively, in a goroutine, and as a
// value that a literal returns. Each local is boxed (ADR-139): nothing else keeps it alive.
package main

import (
	"fmt"
	"sync"
)

import "./cvar_addr_literals.hpp"

type H struct{ p *C.int }

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

func f1() int {
	var a, b H
	var x C.int = 71
	a.p = &x
	func() {
		var y C.int = 72
		b.p = &y
		_ = a
	}()
	clobber()
	return iv(b.p) + iv(a.p)
}

func f2Make() *C.int {
	var x C.int = 73
	var h1, h2 H
	h1.p = &x
	func() { h2 = h1 }()
	return h2.p
}

func f2() int {
	p := f2Make()
	clobber()
	return iv(p)
}

func f3() int {
	var wg sync.WaitGroup
	wg.Add(1)
	var cur *C.int
	go func() {
		defer wg.Done()
		var y C.int = 74
		cur = &y
	}()
	wg.Wait()
	clobber()
	return iv(cur)
}

func f5() int {
	var a *C.int
	func() {
		var b *C.int
		func() {
			var c *C.int
			func() {
				var xc C.int = 77
				c = &xc
			}()
			b = c
		}()
		a = b
	}()
	clobber()
	return iv(a)
}

func f5b() int {
	var a *C.int
	func() {
		var b *C.int
		var xa C.int = 75
		func() {
			var c *C.int
			var xb C.int = 76
			func() {
				var xc C.int = 77
				c = &xc
				b = &xb
				a = &xa
			}()
			_ = c
		}()
		_ = b
	}()
	clobber()
	return iv(a)
}

func f6() int {
	p := func() *C.int {
		var y C.int = 78
		q := &y
		return q
	}()
	clobber()
	return iv(p)
}

func f7Run() {
	var y C.int = 79
	defer func() { gcur = &y }()
}

func f7() int {
	f7Run()
	clobber()
	return iv(gcur)
}

func f8() int {
	var cur *C.int
	set := func(v C.int) {
		var y C.int = v
		cur = &y
	}
	set(80)
	set(81)
	clobber()
	return iv(cur)
}

func f9Rec(n int, out *H) {
	var y C.int = C.int(82 + n)
	if n == 0 {
		out.p = &y
		return
	}
	f9Rec(n-1, out)
}

func f9() int {
	var h H
	f9Rec(2, &h)
	clobber()
	return iv(h.p)
}

func f10Run() {
	var x C.int = 83
	var h H
	h.p = &x
	func() { gcur = h.p }()
}

func f10() int {
	f10Run()
	clobber()
	return iv(gcur)
}

func f11() int {
	func() {
		var y C.int = 84
		var hl H
		hl.p = &y
		hl2 := hl
		gcur = hl2.p
	}()
	clobber()
	return iv(gcur)
}

func f12Make() H {
	return func() H {
		var y C.int = 85
		var hl H
		hl.p = &y
		return hl
	}()
}

func f12() int {
	h := f12Make()
	clobber()
	return iv(h.p)
}

func f13Run() {
	var x C.int = 86
	var cur *C.int
	cur = &x
	func() { gcur = cur }()
}

func f13() int {
	f13Run()
	clobber()
	return iv(gcur)
}

func f14Run() {
	var x C.int = 87
	var cur *C.int
	f := func() { cur = &x }
	f()
	done := make(chan bool)
	go func() {
		gcur2 = cur
		done <- true
	}()
	<-done
}

func f14() int {
	f14Run()
	clobber()
	return iv(gcur2)
}

func f15() int {
	var wg sync.WaitGroup
	wg.Add(1)
	var res int
	var x C.int = 88
	go func(p *C.int) {
		defer wg.Done()
		res = int(*p)
	}(&x)
	wg.Wait()
	return res
}

func f16Run() {
	var x C.int = 89
	h := &H{}
	func() {
		h.p = &x
		gcur = h.p
	}()
}

func f16() int {
	f16Run()
	clobber()
	return iv(gcur)
}

func f17Run() {
	defer cleanup()
	var x C.int = 90
	func() {
		var y C.int = 91
		gcur = &y
		gcur2 = &x
	}()
}

func f17() int {
	f17Run()
	clobber()
	return iv(gcur) + iv(gcur2)
}

func main() {
	fmt.Println(f1(), f2(), f3(), f5(), f5b(), f6())
	fmt.Println(f7(), f8(), f9(), f10(), f11(), f12())
	fmt.Println(f13(), f14(), f15(), f16(), f17())
}

// Output:
// 143 73 74 77 75 78
// 79 81 82 83 84 85
// 86 87 88 89 181
