// phase: 03

// gc copies a fmt argument that needs an address (a bool, an int8, a [1]int8, a small struct)
// into a temporary in its place also when it is a local variable (or a part of one), which a
// closure that changes it can see: fmt.Println(ok, set()) prints the old ok, and an index that a
// call changes: fmt.Println(one[k], setk()) of a [1]bool reads one[k] first. It reads the new
// value when the local is a parameter or a result, its address is taken, a closure that escapes
// captures it (stored, passed, deferred, assigned with =, or only made: _ = f) or one that gc
// can't inline (a defer in its body), or its type needs no address (int, string) or doesn't fit
// in registers (5 fields). gocpp approximates gc's inliner and escape analysis (lower/inplace.go,
// divergence D25); these are the cases where both agree.
package main

import "fmt"

type S16 struct{ a, b int }
type S5 struct{ a, b, c, d, e int8 }

var gfn func() int

func keep(f func() int) { gfn = f }

func param(p bool) {
	set := func() bool { p = true; return p }
	fmt.Println(p, set())
}

func named() (r bool) {
	set := func() bool { r = true; return r }
	fmt.Println(r, set())
	return
}

func main() {
	ok := false
	set := func() bool { ok = true; return ok }
	fmt.Println(ok, set())
	for i := int8(0); i < 2; i++ {
		inc := func() int { i += 5; return 0 }
		fmt.Println(i, inc())
	}
	b, u, a1, t := false, uint8(1), [1]int8{1}, S16{1, 2}
	f := func() int { b, u, a1[0], t = true, 9, 9, S16{9, 9}; return 0 }
	fmt.Println(b, u, a1, t, f())
	// read new: int, string, 5 fields, address taken, escaping closure, deferred
	n, s, s5 := 1, "a", S5{}
	g := func() int { n, s, s5.a = 2, "b", 9; return 0 }
	fmt.Println(n, s, s5, g())
	c := false
	pc := &c
	h := func() int { c = true; return 0 }
	fmt.Println(c, h(), *pc)
	e := false
	esc := func() int { e = true; return 0 }
	keep(esc)
	fmt.Println(e, esc())
	e2 := false
	esc2 := func() int { e2 = !e2; return 0 }
	defer esc2()
	fmt.Println(e2, esc2())
	e3 := false
	fmt.Println(e3, func() int { e3 = true; return 0 }())
	e4 := false
	var k func() int
	k = func() int { e4 = true; return 0 }
	fmt.Println(e4, k())
	e5 := false
	w := func() int { e5 = true; return 0 }
	func() { fmt.Println(e5, w()) }()
	param(false)
	named()
	arr := [2]bool{}
	fa := func() int { arr[0] = true; return 0 }
	fmt.Println(arr, fa())
	made()
	nested()
	notInlined()
	parts()
	indexes()
}

// notInlined: gc never inlines a closure whose own body has a defer (or a go statement, or a
// recover call: Phases 08 and 05), so it captures by reference and reads the new value; a defer
// in a func literal inside the closure doesn't count.
func notInlined() {
	a := false
	fa := func() bool { defer func() {}(); a = true; return a }
	fmt.Println(a, fa())
	e := false
	fmt.Println(e, func() bool { defer func() {}(); e = true; return e }())
	d := false
	fd := func() bool { d = true; func() { defer func() {}() }(); return d }
	fmt.Println(d, fd())
}

// parts: a field, or the element of a [1]T, of a local that gc copies in place is copied too,
// when its own value needs an address (a bool); an int field is read new.
type T struct {
	ok bool
	n  int
}

func parts() {
	t := T{}
	ft := func() int { t.ok, t.n = true, 5; return 0 }
	fmt.Println(t.ok, t.n, t, ft())
	a := [1]T{}
	fa := func() int { a[0].ok, a[0].n = true, 5; return 0 }
	fmt.Println(a[0].ok, a[0].n, a[0], a, fa())
}

// made: _ = flip makes flip a value, so gc captures the locals by reference: both calls read new.
func made() {
	sb, s8 := false, int8(1)
	flip := func(t int) int { sb = !sb; s8 += int8(t); return t }
	_ = flip
	fmt.Println(sb, s8, flip(3), sb)
	flip2 := func(t int) int { sb = !sb; s8 += int8(t); return t }
	fmt.Println(sb, s8, flip2(3), sb)
}

// nested: closures inside closures that are only called, and a range variable.
func nested() {
	ok := false
	outer := func() bool {
		inner := func() bool { ok = !ok; return ok }
		fmt.Println(ok, inner(), ok)
		return inner()
	}
	fmt.Println(ok, outer(), ok)
	for _, v := range []bool{false, true} {
		set := func() int { v = !v; return 0 }
		fmt.Println(v, set(), v)
	}
}

// indexes: gc copies one[k] of a plain local [1]bool in place (one fits in registers), so a call
// that changes only k doesn't change which element prints (and doesn't make it panic); a [2]bool
// doesn't fit, so two[k] is read after the calls, with the new k, as here.
var gk = 0

func setk() int { gk = 3; return 0 }

func indexes() {
	var one [1]bool
	fmt.Println(one[gk], setk())
	gk = 0
	k := 0
	fk := func() int { k = 3; return 0 }
	fmt.Println(one[k], fk())
	type box struct {
		n   int8
		arr [1]bool
	}
	var b box
	fmt.Println(b.arr[gk], setk(), b.n)
	gk = 0
	two := [2]bool{false, true}
	fmt.Println(two[gk&1], setk(), two)
}
