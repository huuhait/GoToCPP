package cppgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// orderPrelude is the start of each TestOrderPanics program: f and grow change the package
// variables x and s, and print, so the output shows when they ran.
const orderPrelude = `package main

import "fmt"

var x = 1
var s = "abc"

func f() int {
	x++
	fmt.Println("f", x)
	return x
}

func grow() int {
	s += "z"
	fmt.Println("grow", len(s))
	return len(s)
}

func id(v int) int {
	fmt.Println("id", v)
	return v
}

`

// sl is a package slice for the slice cases of TestOrderPanics.
const sl = "var gsl = []int{1, 2, 3}\n\n"

// arr is a package array for the array cases of TestOrderPanics.
const arr = "var garr = [3]int{1, 2, 3}\n\n"

// st has the structs for the struct cases of TestOrderPanics: Pt, Wrap (an embedded *Pt), a
// package Pt and a package slice of them.
const st = "type Pt struct{ a, b int }\n\ntype Wrap struct{ *Pt }\n\nvar gst Pt\n\nvar gsts = []Pt{{1, 2}}\n\n"

// nt has the named types for the named cases of TestOrderPanics (step 3.9): a named int, a
// named slice of them, package variables of both, and a function that gives a []N.
const nt = "type N int\n\ntype NS []N\n\nvar gn N = 5\n\nvar gns = NS{1, 2, 3}\n\nfunc mkn() []N {\n\treturn nil\n}\n\n"

// mt has the methods for the method cases of TestOrderPanics (step 3.10): a value and a pointer
// method that print their receiver, a package pointer that nilr sets to nil, and a package slice.
const mt = "type R struct{ v int }\n\nfunc (r R) get(a int) int {\n\tfmt.Println(\"get\", r.v, a)\n\treturn r.v\n}\n\n" +
	"func (r *R) put(a int) {\n\tfmt.Println(\"put\", r.v, a)\n\tr.v = a\n}\n\n" +
	"func (r *R) self(a int) *R {\n\tfmt.Println(\"self\", a)\n\treturn r\n}\n\nvar gr *R\n\nvar grs = []R{{1}}\n\n" +
	"func callr(f func(int) int, n int) int {\n\treturn f(n)\n}\n\nfunc nilr() int {\n\tfmt.Println(\"nilr\")\n\tgr = nil\n\treturn 0\n}\n\nfunc mkr() *R {\n\tfmt.Println(\"mkr\")\n\treturn nil\n}\n\n"

// fv has the func values for the func cases of TestOrderPanics: a package func variable that
// setfv changes, and mkfv, which gives a nil func.
const fv = "var gfv = func(v int) int {\n\tfmt.Println(\"old gfv\", v)\n\treturn v\n}\n\n" +
	"func setfv() int {\n\tgfv = func(v int) int {\n\t\tfmt.Println(\"new gfv\", v)\n\t\treturn v\n\t}\n\treturn 1\n}\n\n" +
	"func mkfv() func(int) {\n\tfmt.Println(\"mkfv\")\n\treturn nil\n}\n\n"

// two has the variables for the cases of TestOrderPanics with two operations that both panic
// (step 3.12): a package slice and array (an index 5 is out of range in both, with another
// length), a nil pointer and a nil map, and r2 and r3, which return two such values with and
// without defers.
const two = "var gs2 = []int{1, 2, 3}\n\nvar ga2 = [2]int{1, 2}\n\ntype P2 struct{ a, b int }\n\nvar gp2 *P2\n\n" +
	"var gm2 map[int]int\n\nfunc g2(a, b int) int {\n\treturn a + b\n}\n\n" +
	"func r2(i int) (int, int) {\n\treturn gs2[i], ga2[i]\n}\n\n" +
	"func r3(i int) (int, int) {\n\tdefer func() {}()\n\treturn gs2[i], ga2[i]\n}\n\n" +
	"func r4(i int) (a, b int) {\n\tdefer func() {}()\n\ta, b = gs2[i], ga2[i]\n\treturn\n}\n\n"

// TestOrderPanics checks the order of calls and of the operations that can panic (order.go)
// against real Go: gc calls f first in a/b + f() and panics after it, and reads s after grow
// in int(s[i]) + grow(). Each program panics, with Go's output and exit code 2, on both
// compilers, also which of two operations that both panic reports its panic (step 3.12).
func TestOrderPanics(t *testing.T) {
	tests := []struct{ name, decls, body string }{
		{"div-call", "", "a, b := 7, 0\n\tfmt.Println(a/b + f())"},
		{"call-div", "", "a, b := 7, 0\n\tfmt.Println(f() + a/b)"},
		{"call-over-zero", "", "z := 0\n\tfmt.Println(f() / z)"},
		{"div-decl", "", "a, b := 7, 0\n\ty, z := a/b, f()\n\tfmt.Println(y, z)"},
		{"div-var", "", "a, b := 7, 0\n\tvar y, z = a / b, f()\n\tfmt.Println(y, z)"},
		{"div-assign", "", "a, b := 7, 0\n\ty := 0\n\ty = a/b + f()\n\tfmt.Println(y)"},
		{"div-parallel", "", "a, b := 7, 0\n\ty, z := 0, 0\n\ty, z = a/b, f()\n\tfmt.Println(y, z)"},
		{"div-for-init", "", "a, b := 7, 0\n\tfor i, j := a/b, f(); i < j; i++ {\n\t}"},
		{"div-return", "func r(z int) (int, int) {\n\treturn 7 / z, f()\n}\n", "fmt.Println(r(0))"},
		{"op-assign", "", "x /= f() - x\n\tfmt.Println(x)"},
		{"index-call", "", "i := 10\n\tfmt.Println(int(s[i]) + f())"},
		{"index-grow", "", "i := 4\n\tfmt.Println(s[i], grow())"},
		{"index-plus-grow", "", "i := 4\n\tfmt.Println(int(s[i]) + grow())"},
		{"shift-call", "", "n := -1\n\tfmt.Println(1<<n + f())"},
		{"nested-div-first", "", "z := 0\n\tfmt.Println(id(7/z), f())"},
		{"nested-call-first", "", "z := 0\n\tfmt.Println(f(), id(7/z))"},
		{"and-unit", "", "z := 0\n\tfmt.Println(f() > 0 && 7/z > 0, f())"},
		{"for-cond", "", "z := 0\n\tfor i := 0; i < 7/z+f(); i++ {\n\t}"},
		{"for-cond-later", "", "z := 3\n\tfor i := 0; i < 7/z+f(); i++ {\n\t\tz--\n\t}"},
		{"else-if", "", "z := 0\n\tif f() > 100 {\n\t} else if 7/z+f() > 0 {\n\t\tfmt.Println(1)\n\t}"},
		{"case", "", "z := 0\n\tswitch f() {\n\tcase 1:\n\tcase 7/z + f():\n\t\tfmt.Println(1)\n\t}"},
		{"case-list", "", "z := 0\n\tswitch f() {\n\tcase 1, id(7/z) + f():\n\t\tfmt.Println(1)\n\t}"},
		// A nil dereference panics after the calls of its scope, like the other operations.
		{"deref-call", "", "var p *int\n\tfmt.Println(*p + f())"},
		{"deref-arg", "", "var p *int\n\tfmt.Println(*p, f())"},
		{"store-call", "", "var p *int\n\t*p = f()"},
		{"op-assign-call", "", "var p *int\n\t*p += f()"},
		{"parallel-store", "", "var p *int\n\ty := 0\n\t*p, y = 1, f()\n\tfmt.Println(y)"},
		{"addr-deref", "", "var p *int\n\tfmt.Println(&*p == nil, f())"},
		// Slices (step 3.5): s[i] panics after the calls of its scope, also in a store; a
		// slice expression, make and &s[i] in a comparison (copied, as a bool argument of
		// fmt) panic in their places, before the calls after them; so does append(s[:j], ...).
		{"slice-index-call", sl, "i := 5\n\tfmt.Println(gsl[i] + f())"},
		{"slice-index-arg", sl, "i := 5\n\tfmt.Println(gsl[i], f())"},
		{"slice-store-call", sl, "i := 5\n\tgsl[i] = f()"},
		{"slice-store-index-call", sl, "gsl[id(7)] = f()"},
		{"slice-op-assign-call", sl, "i := 5\n\tgsl[i] += f()"},
		{"slice-parallel-store", sl, "y := 0\n\tgsl[id(9)], y = f(), f()\n\tfmt.Println(y)"},
		{"slice-expr-first", sl, "j := 5\n\tfmt.Println(len(gsl[:j]), f())"},
		{"slice-expr-bound-call", sl, "fmt.Println(len(gsl[id(1):id(9)]), f())"},
		{"slice3-first", sl, "k := 9\n\tfmt.Println(f(), len(gsl[0:1:k]), f())"},
		{"make-first", "", "n := -1\n\tfmt.Println(len(make([]int, n)), f())"},
		{"make-call-len", "", "fmt.Println(len(make([]int, id(-2))), f())"},
		{"addr-elem-first", sl, "i := 3\n\tfmt.Println(&gsl[i] == nil, f())"},
		{"append-sub-first", sl, "j := 4\n\tfmt.Println(len(append(gsl[:j], f())))"},
		{"literal-index", sl, "i := 7\n\tfmt.Println(len([]int{f(), gsl[i], f()}))"},
		{"range-elem", sl, "for i := range 4 {\n\t\tfmt.Println(gsl[i], f())\n\t}"},
		// Arrays (step 3.6): a[i] and p[i] (a nil p) panic after the calls of their scope, also
		// in a store and op=, and so do [N]T(s) and (*[N]T)(s); a slice expression and &a[i]
		// in a comparison panic in their places, before the calls after them.
		{"array-index-call", arr, "i := 5\n\tfmt.Println(garr[i] + f())"},
		{"array-index-arg", arr, "i := 5\n\tfmt.Println(garr[i], f())"},
		{"array-local-index-arg", "", "a, i := [2]bool{}, 2\n\tfmt.Println(a[i], f())"},
		{"array-store-call", arr, "i := 5\n\tgarr[i] = f()"},
		{"array-store-index-call", arr, "garr[id(7)] = f()"},
		{"array-op-assign-call", arr, "i := 5\n\tgarr[i] += f()"},
		{"array-parallel-store", arr, "y := 0\n\tgarr[id(9)], y = f(), f()\n\tfmt.Println(y)"},
		{"array-ptr-call", "", "var p *[3]int\n\tfmt.Println(p[0], f())"},
		{"array-ptr-store-call", "", "var p *[3]int\n\tp[0] = f()"},
		{"array-to-array-call", "", "s := []int{1}\n\tfmt.Println([3]int(s)[0], f())"},
		{"array-to-ptr-call", "", "s := []int{1}\n\tfmt.Println((*[3]int)(s)[0], f())"},
		{"array-slice-first", arr, "j := 5\n\tfmt.Println(len(garr[:j]), f())"},
		{"array-slice-ptr-first", "", "var p *[3]int\n\tfmt.Println(len(p[:]), f())"},
		{"array-addr-first", arr, "i := 3\n\tfmt.Println(&garr[i] == nil, f())"},
		{"array-literal-index", arr, "i := 7\n\tfmt.Println(len([3]int{f(), garr[i], f()}))"},
		{"array-range-ptr", "", "var p *[3]int\n\tfor _, v := range p {\n\t\tfmt.Println(v, f())\n\t}"},
		// Maps (step 3.7): the key's calls, then the value (with its panics), before the entry
		// is taken, which panics for a nil map; m[k] is read after the calls; a map literal
		// stores its entries one by one (a panic in an entry comes before the calls of the
		// entries after it, where a slice literal calls them first).
		{"map-nil-store-calls", "", "var m map[int]int\n\tm[id(1)] = f()"},
		{"map-store-value-first", "", "var m map[int]int\n\tz := 0\n\tm[f()] = 7 / z"},
		{"map-op-assign-call", "", "m := map[int]int{}\n\tm[id(1)] /= f() - x"},
		{"map-nil-op-assign-call", "", "var m map[int]int\n\tm[id(1)] += f()"},
		{"map-read-deref", "", "var p *map[int]int\n\tfmt.Println((*p)[1], f())"},
		{"map-literal-entry", "", "z := 0\n\t_ = map[int]int{f(): 7 / z, id(2): 1}"},
		{"map-literal-entries", "", "z := 0\n\t_ = map[int]int{1: 7 / z, f(): 1}"},
		{"slice-literal-elems", "", "z := 0\n\t_ = []int{7 / z, f()}"},
		{"map-literal-later", "", "z := 0\n\tfmt.Println(len(map[int]int{1: 7 / z}), f())"},
		{"map-append-call", "", "var m map[int][]int\n\tk := 1\n\tm[k] = append(m[k], f(), id(2))"},
		// m[k] = append(m[k], ...) with the same m and k that are not names: gc evaluates m and
		// k (and their panics) before the other arguments.
		{"map-append-nil-ptr", "", "var p *map[int][]int\n\tk := 1\n\t(*p)[k] = append((*p)[k], f())"},
		{"map-append-index", "", "ss := []map[int][]int{{}}\n\ti := 3\n\tss[i][0] = append(ss[i][0], f())"},
		{"map-append-key-div", "", "m := map[int][]int{}\n\tz := 0\n\tm[1/z] = append(m[1/z], f())"},
		{"map-append-key-call", "", "var m map[int][]int\n\tm[id(1)+0] = append(m[id(1)+0], f())"},
		// Structs (step 3.8): a field through a nil pointer (also an embedded one) panics after
		// the calls of its scope, also in a store, op= and a keyed literal (whose values Go
		// evaluates in source order); &p.f in a comparison (copied) panics in its place; an
		// index in a field of a package struct panics after the calls.
		{"struct-ptr-call", st, "var p *Pt\n\tfmt.Println(p.a + f())"},
		{"struct-ptr-arg", st, "var p *Pt\n\tfmt.Println(p.b, f())"},
		{"struct-ptr-store", st, "var p *Pt\n\tp.a = f()"},
		{"struct-ptr-op-assign", st, "var p *Pt\n\tp.b -= f()"},
		{"struct-ptr-parallel", st, "var p *Pt\n\ty := 0\n\tp.a, y = 1, f()\n\tfmt.Println(y)"},
		{"struct-embedded-arg", st, "var w Wrap\n\tfmt.Println(w.b, f())"},
		{"struct-addr-first", st, "var p *Pt\n\tfmt.Println(&p.a == nil, f())"},
		{"struct-keyed", st, "var p *Pt\n\tfmt.Println(Pt{b: f(), a: p.b + f()})"},
		{"struct-keyed-order", st, "var p *Pt\n\tq := Pt{b: id(f()), a: id(p.a)}\n\tfmt.Println(q)"},
		{"struct-slice-elem", st, "i := 4\n\tfmt.Println(gsts[i].a, f())"},
		{"struct-field-index", st + "var garrs struct{ a [3]int }\n\n", "i := 5\n\tfmt.Println(garrs.a[i], f())"},
		// defer (step 3.13): the arguments of a deferred call are evaluated at the defer
		// statement, in the order of a call: the calls first, then the panics; a panic there
		// runs the calls deferred before it, and the deferred calls see the values of then.
		{"defer-div-call", "", "a, b := 7, 0\n\tdefer fmt.Println(a/b + f())"},
		{"defer-index-arg", sl, "i := 5\n\tdefer fmt.Println(\"first\")\n\tdefer fmt.Println(gsl[i], f())"},
		{"defer-deref-arg", "", "var p *int\n\tdefer id(f())\n\tdefer id(*p + f())"},
		{"defer-runs-later", "", "defer fmt.Println(\"deferred\", x, f())\n\tx = 100\n\tz := 0\n\tfmt.Println(7 / z)"},
		{"defer-map-nil", "", "var m map[int]int\n\tdefer fmt.Println(\"after\", f())\n\tm[id(1)] = f()"},
		{"defer-panic-arg", "", "defer fmt.Println(\"first\", f())\n\tdefer panic(id(f()))\n\tfmt.Println(\"body\", x)"},
		// Named types (step 3.9): a division, a shift and an index of named types panic after
		// the calls of their scope, like those of their underlying types; a slice expression of
		// a named slice in its place among the calls.
		{"named-div-call", nt, "z := N(0)\n\tfmt.Println(gn/z + N(f()))"},
		{"named-op-assign", nt, "gn /= N(f() - x)\n\tfmt.Println(gn)"},
		{"named-shift-call", nt, "k := -1\n\tfmt.Println(gn<<k, f())"},
		{"named-index-call", nt, "i := 5\n\tfmt.Println(gns[i], f())"},
		{"named-index-named", nt, "var i N = 7\n\tfmt.Println(int(gns[i])+f(), f())"},
		{"named-slice-first", nt, "j := 9\n\tfmt.Println(len(gns[:j]), f())"},
		{"named-append-index", nt, "gns = append(gns[:1], N(f()))\n\tfmt.Println(gns[2], f())"},
		// Methods (step 3.10): the receiver is evaluated with the arguments, as gc does: its
		// calls before theirs, its reads after all of them (a nil gr after nilr, an index out
		// of range after f), also for a deferred call, whose receiver is stored at the defer.
		{"method-recv-after-call", mt, "gr = &R{1}\n\tgr.get(nilr())"},
		{"method-recv-call-first", mt, "mkr().get(id(1))"},
		{"method-ptr-recv-call-first", mt, "mkr().put(id(1))"},
		{"method-index-recv", mt, "i := 3\n\tgrs[i].get(f())"},
		{"method-index-ptr-recv", mt, "i := 3\n\tgrs[i].put(f())"},
		{"method-defer-recv", mt, "var p *R\n\tdefer fmt.Println(\"first\")\n\tdefer p.get(f())"},
		{"method-arg-panic", mt, "z := 0\n\tgr = &R{2}\n\tgr.get(7 / z + f())"},
		{"method-chain", mt, "gr = &R{3}\n\tfmt.Println(gr.self(id(1)).get(nilr()), gr.self(id(2)).get(f()))"},
		// Method values (step 3.10b): the receiver is evaluated where the value is made, after
		// the calls of its scope (gr.get next to nilr() binds the nil gr), a call in it first; a
		// value receiver is copied then, and the later changes don't reach it.
		{"method-value-recv-after-call", mt, "gr = &R{1}\n\tfmt.Println(callr(gr.get, nilr()))"},
		{"method-value-call-first", mt, "fmt.Println(callr(mkr().get, id(1)))"},
		{"method-value-index", mt, "i := 3\n\tfmt.Println(callr(grs[i].get, f()))"},
		{"method-value-copy", mt, "gr = &R{1}\n\tg, p := gr.get, gr.put\n\tgr.v = 5\n\tp(7)\n\tfmt.Println(g(0), gr.v)\n\tgr = nil\n\tfmt.Println(g(1), callr(gr.get, 2))"},
		// Func values (step 3.11): a func value is read after the calls of its call's scope (a
		// package variable that an argument changes: the new func), a call that gives one runs
		// before the arguments, a nil func panics after them, an element of a slice of funcs
		// panics after them, and a closure reads what it captured when it runs.
		{"func-pkg-var", fv, "fmt.Println(gfv(setfv()))\n\tpanic(\"end\")"},
		{"func-call-first", fv, "mkfv()(id(1))"},
		{"func-nil-after-args", "", "var fn func(int, int)\n\tfn(f(), id(2))"},
		{"func-elem-after-args", fv, "fs := []func(int) int{gfv}\n\ti := 3\n\tfmt.Println(fs[i](f()))"},
		{"func-closure-reads", "", "y := 1\n\tg := func() int { return y }\n\ty = 2\n\tfmt.Println(g(), f())\n\tpanic(g())"},
		{"func-counter", "", "c := 0\n\tnext := func() int {\n\t\tc++\n\t\treturn c\n\t}\n\tfmt.Println(next(), next()*10, next())\n\tpanic(c)"},
		{"func-defer-value", fv, "defer gfv(f())\n\tgfv = nil\n\tfmt.Println(\"body\")\n\tpanic(\"end\")"},
		{"func-defer-pkg-var", fv, "defer gfv(setfv())\n\tpanic(\"end\")"},
		{"func-elem-call-index", fv, "fs := []func(int) int{gfv, gfv}\n\tfs[id(1)](func() int { fs[1] = nil; return 1 }())\n\tpanic(\"end\")"},
		{"func-elem-index", fv, "fs := []func(int) int{gfv, gfv}\n\ti := 1\n\tfs[i](func() int { fs[1] = nil; return 1 }())"},
		{"func-defer-closure-args", "", "defer func(a, b int) { fmt.Println(\"deferred\", a, b, x) }(f(), id(x))\n\tx = 100\n\tpanic(\"end\")"},
		// Two operations that panic (step 3.12): Go reports the one it evaluates first, from
		// left to right, also where C++ leaves the order open (arguments, operands, keyed
		// struct literals); a unit (a slice expression, &&) in its place among the calls,
		// before the others; a func value after its arguments; in x = v, v before the target's
		// operands, and the value of a map store before its key (but not for /= and %=); the
		// values of an assignment with several targets as gc's ascompatee orders them (early,
		// before the stores, when a store before them can change them, else late, in their
		// own store), also a return in a function with defers and a nil map's store.
		{"two-args", two, "i := 5\n\tfmt.Println(gs2[i], ga2[i])"},
		{"two-call-args", two, "i := 5\n\tg2(gs2[i], ga2[i])"},
		{"two-plus", two, "i := 5\n\tfmt.Println(gs2[i] + ga2[i])"},
		{"two-compare", two, "i := 5\n\tfmt.Println(ga2[i] == gs2[i])"},
		{"two-div-index", two, "i, z := 5, 0\n\tfmt.Println(gs2[0]/z + ga2[i])"},
		{"two-shift", two, "i, n := 5, -1\n\tfmt.Println(gs2[0]<<n+ga2[i], gs2[i])"},
		{"two-deref", two, "i := 5\n\tvar p *int\n\tfmt.Println(gs2[i] + *p)"},
		{"two-field", two, "i := 5\n\tfmt.Println(gs2[i], gp2.b)"},
		{"two-unary", two, "i := 5\n\tfmt.Println(-gs2[i] + ^ga2[i])"},
		{"two-conv", two, "i := 5\n\tfmt.Println(float64(gs2[i]) + float64(ga2[i]))"},
		{"two-keyed-struct", two, "i := 5\n\tfmt.Println(P2{b: gs2[i], a: ga2[i]})"},
		{"two-map-literal", two, "i := 5\n\tfmt.Println(map[int]int{gs2[i]: ga2[i]})"},
		{"two-and", two, "i := 5\n\tfmt.Println(x > 0 && ga2[i] > 0, gs2[i])"},
		{"two-or-right", two, "i := 5\n\tfmt.Println(f() > 9 || gs2[i] > ga2[i])"},
		{"two-slice-expr", two, "i := 5\n\tfmt.Println(gs2[i:], ga2[i])"},
		{"two-slice-expr-later", two, "i := 5\n\tfmt.Println(ga2[i], gs2[i:])"},
		{"two-substrings", two, "i := 5\n\tfmt.Println(s[i:] < s[:i])"},
		{"two-append", two, "i := 5\n\tgs2 = append(gs2, gs2[i], ga2[i])"},
		{"two-copy", two, "i := 5\n\tcopy(gs2[i:], ga2[i:])"},
		{"two-min", two, "i := 5\n\tfmt.Println(min(gs2[i], ga2[i]))"},
		{"two-println", two, "i := 5\n\tprintln(gs2[i], ga2[i])"},
		{"two-printf", two, "i := 5\n\tfmt.Printf(\"%d %d\\n\", gs2[i], ga2[i])"},
		{"two-defer", two, "i := 5\n\tdefer fmt.Println(gs2[i], ga2[i])"},
		// A deferred call evaluates its func value and its receiver first (gc copies them, then
		// the arguments, into new variables), unlike a call.
		{"two-defer-func-value", two, "i, j := 5, 7\n\tfs := []func(int) int{id}\n\tdefer fs[i](gs2[j])"},
		{"two-defer-value-recv", two + mt, "i := 5\n\tdefer grs[i].get(gs2[i])"},
		{"two-defer-nil-recv", two + mt, "i := 5\n\tvar p *R\n\tdefer p.get(gs2[i])"},
		{"two-defer-pkg-nil-recv", two + mt, "i := 5\n\tdefer gr.get(gs2[i])"},
		{"two-defer-call-recv", two + mt, "i := 5\n\tdefer mkr().get(gs2[i])"},
		{"two-defer-named-recv", two + nt + "func (n N) nv(a int) int {\n\treturn int(n) + a\n}\n\n", "i := 5\n\tdefer gns[i].nv(ga2[i])"},
		{"two-op-assign-bound", two, "i, k := 5, 0\n\tc := [][]int{{1}}\n\tc[0][k] += gs2[k] * ga2[k]\n\tfmt.Println(c)\n\tc[0][i] += gs2[k] * ga2[i]"},
		{"two-method-ptr-recv", two + mt, "i := 5\n\tgrs[i].put(ga2[i])"},
		{"two-func-value", two, "i := 5\n\tfs := []func(int) int{id}\n\tfmt.Println(fs[i](ga2[i]))"},
		{"two-func-nil", two, "i := 5\n\tvar fn func(int, int) int\n\tfn(gs2[i], ga2[i])"},
		{"two-addr", two, "i := 5\n\tfmt.Println(&gs2[i] == &ga2[i])"},
		{"two-return", two, "fmt.Println(r2(5))"},
		{"two-return-defer", two, "fmt.Println(r3(5))"},
		{"two-named-defer", two, "fmt.Println(r4(5))"},
		{"two-store", two, "i := 5\n\tgs2[i] = ga2[i]"},
		{"two-store-operand", two, "i := 5\n\tgs2[ga2[i]] = gs2[i+1]"},
		// gc copies the parts of a target that are not a name, a field, *p or an index first
		// (walkAssign's safeExpr), also in the late stores of several targets.
		{"two-store-copied", two, "i := 5\n\tgs2[ga2[i]&1] = gs2[i+1]"},
		{"two-store-copied-deref", two, "i := 5\n\tps := []*int{nil}\n\t*ps[ga2[i]&1] = gs2[i+1]"},
		// A call in a copied part runs once (it is a temporary of the order); the panics of a
		// negation, a complement and a conversion are copied too, a conversion to the same type
		// isn't one; a map read in a target is a copied call; x op= y and x++ copy first.
		{"two-copied-call-once", two, "k := 0\n\tgs2[(8*f())&1], k = 1, 2\n\tfmt.Println(gs2, k)\n\tpanic(\"end\")"},
		{"two-copied-call-once-late", two, "k := 0\n\tk, gs2[(8*f())&1] = 1, 2\n\tfmt.Println(gs2, k)\n\tpanic(\"end\")"},
		{"two-copied-call-once-array", two, "k := 0\n\tga2[(8*f())&1], ga2[k&1] = 1, 2\n\tfmt.Println(ga2)\n\tpanic(\"end\")"},
		{"two-store-copied-neg", two, "i := 5\n\tgs2[-ga2[i]] = gs2[i+1]"},
		{"two-store-copied-not", two, "i := 5\n\tgs2[^ga2[i]] = gs2[i+1]"},
		{"two-store-copied-conv", two, "i := 5\n\tu := []uint{1}\n\tgs2[int(u[i])] = gs2[i+1]"},
		{"two-store-same-conv", two, "i := 5\n\tu := []uint{1}\n\tgs2[uint(u[i])] = gs2[i+1]"},
		{"two-store-copied-map-read", two, "i := 5\n\tms := map[int][]int{}\n\tms[ga2[i]][0] = gs2[i+1]"},
		{"two-store-copied-map-key", two, "i := 5\n\tgs2[gm2[ga2[i]]] = gs2[i+1]"},
		{"two-multi-copied-map-read", two, "i, k := 5, 0\n\tms := map[int][]int{}\n\tk, ms[ga2[i]][0] = 1, gs2[i+1]\n\tfmt.Println(k)"},
		{"two-op-assign-copied", two, "i := 5\n\tvar np *struct{ arr [2]int }\n\tnp.arr[ga2[i]&1] += gs2[i+1]"},
		{"two-inc-copied", two, "i := 5\n\tvar np *struct{ arr [2]int }\n\tnp.arr[ga2[i]&1]++"},
		{"two-op-assign-copied-index", two, "i := 5\n\tvar ps []*struct{ arr [2]int }\n\tps[0].arr[ga2[i]&1] += 1"},
		// gc's walk makes a map read and a string ordering comparison runtime calls, whose
		// arguments that can panic it evaluates first; fmt's arguments are such calls too, in
		// order. A key of type int8 (and bool, floats, arrays, structs) is copied among the calls.
		{"two-map-read-key", two, "i := 5\n\tx := gs2[i+1] + gm2[ga2[i]]\n\tfmt.Println(x)"},
		{"two-map-read-key-div", two, "z := 0\n\tx := gs2[6] + gm2[ga2[0]/z]\n\tfmt.Println(x)"},
		{"two-map-read-fmt", two, "i := 5\n\tfmt.Println(gs2[i+1], gm2[ga2[i]])"},
		{"two-string-less", two, "i := 5\n\tss := []string{\"a\"}\n\tfmt.Println(gs2[i+1] == 0 == (ss[i] < \"b\"))"},
		{"two-string-equal", two, "i := 5\n\tss := []string{\"a\"}\n\tfmt.Println(gs2[i+1] == 0 == (ss[i] == \"b\"))"},
		{"two-slow-key", two, "i := 5\n\tmb := map[int8]int{}\n\tfmt.Println(gs2[i+1] + mb[int8(ga2[i])] + f())"},
		// A key of a slow map call that is addressable is read where the map is, after the calls
		// (addrTemp copies only the others); an array key of 8 bytes is always copied.
		{"slow-key-addressable", two, "i := 5\n\tmb := map[bool]int{}\n\tbs := []bool{true}\n\tfmt.Println(mb[bs[i]] + f())"},
		{"slow-key-addressable-store", two, "i := 5\n\tmb := map[bool]int{}\n\tbs := []bool{true}\n\tmb[bs[i]] = f()"},
		{"slow-key-addressable-op-assign", two, "i := 5\n\tm8 := map[int8]int{}\n\tks := []int8{1}\n\tm8[ks[i]] += f()"},
		{"slow-key-addressable-literal", two, "i := 5\n\tbs := []bool{true}\n\tfmt.Println(map[bool]int{bs[i]: f()})"},
		{"slow-key-big-value", two, "i := 5\n\tmbig := map[int][17]int{}\n\tfmt.Println(mbig[gs2[i]][0] + f())"},
		{"slow-key-pkg-var", "var gbk bool\n\nvar mbk = map[bool]int{false: 10, true: 20}\n\nfunc flipk() int {\n\tgbk = !gbk\n\treturn 1\n}\n\n",
			"fmt.Println(mbk[gbk] + flipk())\n\tmbk[gbk] = flipk()\n\tfmt.Println(mbk)\n\tpanic(\"end\")"},
		// A local that gc keeps in registers is copied too, also when a closure that gc inlines
		// changes it (inplace.go, D25); not for a fast map, nor when the closure escapes.
		{"slow-key-closure-local", "", "mbl := map[bool]int{false: 10, true: 20}\n\tlb := false\n\tflip := func() int {\n\t\tlb = !lb\n\t\treturn 1\n\t}\n\tfmt.Println(mbl[lb] + flip())\n\tx := mbl[lb] + flip()\n\tfmt.Println(x)\n\tpanic(\"end\")"},
		{"fast-key-closure-local", "", "mi := map[int]int{0: 10, 1: 20}\n\tli := 0\n\tinc := func() int {\n\t\tli++\n\t\treturn 1\n\t}\n\tfmt.Println(mi[li] + inc())\n\tpanic(\"end\")"},
		{"slow-key-closure-escapes", "var gfk func() int\n\n", "mbl := map[bool]int{false: 10, true: 20}\n\tlb := false\n\tgfk = func() int {\n\t\tlb = !lb\n\t\treturn 1\n\t}\n\tfmt.Println(mbl[lb] + gfk())\n\tpanic(\"end\")"},
		{"fast-cast-key", two, "i := 5\n\tm := map[[2]int32]int{}\n\tks := [][2]int32{{}}\n\tfmt.Println(m[ks[i]] + f())"},
		{"two-slow-key-store", two, "i := 5\n\tmb := map[bool]int{}\n\tbs := []bool{true}\n\tmb[!bs[i]] = gs2[i+1] + f()"},
		{"two-multi-store-copied", two, "i, b := 5, 0\n\tgs2[ga2[i]&1], b = gs2[i+1], 1\n\tfmt.Println(b)"},
		{"two-op-assign", two, "i := 5\n\tgs2[i] += ga2[i]"},
		{"two-map-store", two, "i := 5\n\tm := map[int]int{}\n\tm[gs2[i]] = ga2[i]"},
		{"two-map-op-assign", two, "i := 5\n\tm := map[int]int{}\n\tm[gs2[i]] += ga2[i]"},
		{"two-map-div-assign", two, "i := 5\n\tm := map[int]int{}\n\tm[gs2[i]] /= ga2[i]"},
		{"two-multi-pkg", two, "i := 5\n\tga2[i], gs2[i] = ga2[i+1], gs2[i+1]"},
		{"two-multi-local", two, "i := 5\n\ta, b := 0, 0\n\ta, b = gs2[i], ga2[i]\n\tfmt.Println(a, b)"},
		{"two-multi-boxed", two, "i := 5\n\ta, b := gs2[i], ga2[i]\n\tfmt.Println(&a != nil, b)"},
		{"two-multi-map-nil", two, "i := 5\n\ta := 0\n\tgm2[gs2[0]], a = ga2[i], 1\n\tfmt.Println(a)"},
		{"two-multi-map-key", two, "i := 5\n\tx, gm2[gs2[i]] = ga2[i], 1"},
		{"two-multi-ptr", two, "i := 5\n\tvar p *int\n\t*p, x = 1, ga2[i]"},
		{"two-multi-field", two, "i := 5\n\tvar ps []*P2\n\tps[0].a, ps[i].b = 1, 2"},
		{"two-shadow", two, "z, p := 0, 1\n\t{\n\t\tv, p := 7/z, ga2[p+4]\n\t\tfmt.Println(v, p)\n\t}"},
		{"two-else-if", two, "i := 5\n\tif f() > 100 {\n\t} else if gs2[i] > ga2[i] {\n\t}"},
		{"two-case", two, "i := 5\n\tswitch f() {\n\tcase 1:\n\tcase gs2[i] + ga2[i]:\n\t}"},
		{"two-switch-tag", two, "i := 5\n\tswitch gs2[i] + ga2[i] {\n\t}"},
		{"two-for-cond", two, "i := 5\n\tfor gs2[i] > ga2[i] {\n\t}"},
		{"two-for-post", two, "i := 5\n\tfor j := 0; j < 3; j += gs2[i] + ga2[i] {\n\t\tfmt.Println(j)\n\t}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			src := orderPrelude + tt.decls + "func main() {\n\t" + tt.body + "\n}\n"
			gp := filepath.Join(dir, "main.gp")
			if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			want := goResult(t, gp)
			if want != nil && want.code != 2 {
				t.Fatalf("with Go, the program exits with %d, not with a panic:\n%s", want.code, want.stderr)
			}
			checkRunsNorm(t, emitter(gp), want, maskPCAddr)
		})
	}
}

// TestOrderTemps checks where order.go puts temporaries, and where it doesn't: a call gets one
// only next to an operand that is not a constant or a local variable, and that C++ doesn't
// evaluate after it. (TestEmit runs the same cases in testdata/emit/order.gp against Go.)
func TestOrderTemps(t *testing.T) {
	tests := []struct{ body, want string }{
		// No temporaries.
		{"fmt.Println(f())", "gp::fmt::Println(f());"},
		{"l := 1\n\tg(l, f())", "g(l, f());"},
		{"g(1, f())", "g(1, f());"},
		{"l := 2\n\ty := f() + l*2\n\tfmt.Println(y)", "int64_t y = f() + l * 2;"},
		{"fmt.Println(g(f(), 1) / 3)", "gp::fmt::Println(g(f(), 1) / 3);"},
		{"z := 0\n\tfmt.Println(f() / z)", "gp::fmt::Println(gp::div(f(), z));"},
		{"if f() > 0 && f() < 9 {\n\t}", "if (f() > 0 && f() < 9) {"},
		{"for f() < 3 {\n\t}", "while (f() < 3) {"},
		{"a, b := f(), f()\n\tfmt.Println(a, b)", "int64_t a = f();"},
		{"fmt.Println(x)", "gp::fmt::Println(x);"},
		{"var b bool\n\tfmt.Println(b, f())", "gp::fmt::Println(b, f());"},
		// Temporaries.
		{"fmt.Println(f(), f())", "{ auto gp_t0 = f(); gp::fmt::Println(gp_t0, f()); }"},
		{"g(x, f())", "{ auto gp_t0 = f(); g(x, gp_t0); }"},
		{"g(f(), x)", "{ auto gp_t0 = f(); g(gp_t0, x); }"},
		{"y := x + f()\n\tfmt.Println(y)", "auto gp_t0 = f(); int64_t y = x + gp_t0;"},
		{"x += f()", "{ auto gp_t0 = f(); x = x + gp_t0; }"},
		{"a, z := 7, 0\n\tfmt.Println(a/z + f())", "{ auto gp_t0 = f(); gp::fmt::Println(gp::div(a, z) + gp_t0); }"},
		{"for x < f() {\n\t}", "auto gp_t0 = f(); if (!(x < gp_t0)) break;"},
		{"if f() > 0 && g(x, f()) > 0 {\n\t}", "bool gp_t0 = f() > 0; if (gp_t0) { auto gp_t1 = f(); gp_t0 = g(x, gp_t1) > 0; }"},
		{"s, i, j := \"ab\", 1, 0\n\tfmt.Println(s[i], s[j], s[i] == s[j])", "{ auto gp_t0 = s[i]; auto gp_t1 = s[j]; auto gp_t2 = s[i]; gp::fmt::Println(gp_t0, gp_t1, gp_t2 == s[j]); }"},
		{"a, b := 7, 0\n\tfmt.Println(a/b, 1<<b, a%b)", "{ auto gp_t0 = gp::div(a, b); auto gp_t1 = gp::shl(int64_t(1), b); gp::fmt::Println(gp_t0, gp_t1, gp::rem(a, b)); }"},
		{"fmt.Println(x == 2, f())", "{ auto gp_t0 = x == 2; gp::fmt::Println(gp_t0, f()); }"},
		{"println(x == 2, f())", "{ auto gp_t0 = f(); gp::println(x == 2, gp_t0); }"},
		// Pointers (step 3.3): a boxed variable and *p are read after the calls, like x; a
		// local pointer itself is not.
		{"l := 1\n\t_ = &l\n\tfmt.Println(l, f())", "{ auto gp_t0 = f(); gp::fmt::Println((*l), gp_t0); }"},
		{"l := 1\n\t_ = &l\n\tfmt.Println(l == 1, f())", "{ auto gp_t0 = (*l) == 1; gp::fmt::Println(gp_t0, f()); }"},
		{"p := &x\n\tfmt.Println(*p, f())", "{ auto gp_t0 = f(); gp::fmt::Println(*p, gp_t0); }"},
		{"p := &x\n\tg(*p, 1)", "g(*p, 1);"},
		{"p := &x\n\t*p = f()", "*p = f();"},
		{"p := &x\n\t*p += f()", "{ auto gp_t0 = f(); *p = *p + gp_t0; }"},
		{"p := &x\n\tq := &p\n\t*p = f()\n\t_ = q", "{ auto gp_t0 = f(); *(*p) = gp_t0; }"},
		{"p := &x\n\t*h(p) += 1", "{ auto gp_t0 = gpkg_main::h(p); *gp_t0 = *gp_t0 + 1; }"},
		{"p := &x\n\t*h(p) = f()", "{ auto gp_t0 = gpkg_main::h(p); *gp_t0 = f(); }"},
		{"p := &x\n\t(*h(p))++", "(*gpkg_main::h(p))++;"},
		{"p, q := &x, new(int)\n\t*p, *q = *q, *p", "{ auto gp_t0 = *p; *p = *q; *q = gp_t0; }"},
		{"p, q := &x, new(int)\n\tp, *p = q, 1", "{ auto gp_t0 = p; p = q; *gp_t0 = 1; }"},
		{"l := 1\n\tp := &l\n\tswitch l {\n\tcase f():\n\t}\n\t_ = p", "auto gp_tag0 = (*l);"},
		// Slices (step 3.5): an element is read after the calls (a call can change it through
		// another slice of its array), also one of a local slice and a byte, which is not
		// copied as an argument of fmt (it is addressable); len and cap of a package slice
		// are evaluated in their places; s = append(s, x) and s = s[a:b] move s when nothing
		// else in the call reads it.
		{"ls := []int{1}\n\tfmt.Println(ls[0], f())", "{ auto gp_t0 = f(); gp::fmt::Println(ls[0], gp_t0); }"},
		{"ls := []int{1}\n\tfmt.Println(ls[0], x)", "gp::fmt::Println(ls[0], x);"},
		{"b := []byte(\"x\")\n\tfmt.Println(b[0], f())", "{ auto gp_t0 = f(); gp::fmt::Println(b[0], gp_t0); }"},
		{"b := []byte(\"x\")\n\tfmt.Println(b[0] == 1, f())", "{ auto gp_t0 = b[0] == 1; gp::fmt::Println(gp_t0, f()); }"},
		{"b := []byte(\"x\")\n\tfmt.Println(string(b), f())", "{ auto gp_t0 = f(); gp::fmt::Println(gp::to_string(b), gp_t0); }"},
		{"str := \"x\"\n\tfmt.Println([]byte(str)[0], f())", "{ auto gp_t0 = f(); gp::fmt::Println(gp::to_bytes(str)[0], gp_t0); }"},
		{"fmt.Println(len(gsl), f())", "{ auto gp_t0 = gp::len(gsl); gp::fmt::Println(gp_t0, f()); }"},
		{"fmt.Println(cap(gsl) - len(gsl), f())", "{ auto gp_t0 = gp::cap(gsl); auto gp_t1 = gp::len(gsl); gp::fmt::Println(gp_t0 - gp_t1, f()); }"},
		{"ls := []int{1}\n\tfmt.Println(len(ls), f())", "gp::fmt::Println(gp::len(ls), f());"},
		{"ls := []int{f(), f()}\n\t_ = ls", "gp::slice<int64_t> ls = gp::slice_of<int64_t>({f(), f()});"},
		{"ls := []int{f(), x, f() + x}\n\t_ = ls", "auto gp_t0 = f(); auto gp_t1 = f(); gp::slice<int64_t> ls = gp::slice_of<int64_t>({gp_t0, x, gp_t1 + x});"},
		{"ls := [][]int{{f()}, {1, f()}}\n\t_ = ls", "gp::slice<gp::slice<int64_t>> ls = gp::slice_of<gp::slice<int64_t>>({gp::slice_of<int64_t>({f()}), gp::slice_of<int64_t>({1, f()})});"},
		{"ls := []int{1: f(), 0: f()}\n\t_ = ls", "gp::slice<int64_t> ls = gp::slice_keyed<int64_t>(2, {{1, f()}, {0, f()}});"},
		{"ls := []int{x, f()}\n\t_ = ls", "auto gp_t0 = f(); gp::slice<int64_t> ls = gp::slice_of<int64_t>({x, gp_t0});"},
		{"ls := []int{5: f()}\n\t_ = ls", "gp::slice<int64_t> ls = gp::slice_keyed<int64_t>(6, {{5, f()}});"},
		{"gsl[0] = f()", "{ auto gp_t0 = f(); gsl[0] = gp_t0; }"},
		{"ls := []int{1}\n\tls[f()] = f()", "{ auto gp_t0 = f(); ls[gp_t0] = f(); }"},
		{"ls := []int{1}\n\tls[f()] += 1", "{ auto gp_t0 = f(); ls[gp_t0] = ls[gp_t0] + 1; }"},
		{"ls := []int{1}\n\tls[0]++", "ls[0]++;"},
		{"ls := []int{1}\n\tls, ls[0] = nil, 2", "{ auto gp_t0 = ls; ls = nullptr; gp_t0[0] = 2; }"},
		{"ls := []int{1}\n\ti := 0\n\ti, ls[i] = 1, 2", "{ auto gp_t0 = i; i = 1; ls[gp_t0] = 2; }"},
		{"ls := []int{1}\n\tls[0], ls[0] = ls[0], f()", "{ auto gp_t0 = f(); ls[0] = ls[0]; ls[0] = gp_t0; }"},
		{"ls := []int{1}\n\tls = append(ls, 1)", "ls = gp::append(static_cast<gp::slice<int64_t>&&>(ls), 1);"},
		{"ls := []int{1}\n\tls = append(ls, f())", "ls = gp::append(static_cast<gp::slice<int64_t>&&>(ls), f());"},
		{"ls := []int{1}\n\tls = append(ls, ls[0])", "ls = gp::append(ls, ls[0]);"},
		{"ls := []int{1}\n\tls = append(ls, ls...)", "ls = gp::append_slice(ls, ls);"},
		{"ls := []int{1}\n\tls = ls[1:]", "ls = gp::sub(static_cast<gp::slice<int64_t>&&>(ls), 1);"},
		{"ls := []int{1}\n\tls = ls[:len(ls)-1]", "ls = gp::sub(ls, 0, gp::len(ls) - 1);"},
		{"ls := []int{1}\n\tls = ls[:]", "ls = ls;"},
		{"gsl = gsl[1:]", "gsl = gp::sub(gsl, 1);"},
		// An append to the same variable is in place (not a unit), so a package [][]int moves
		// its row too; a temporary that is only stored is moved into its target.
		{"var k int\n\tgg[k] = append(gg[k], k)", "gg[k] = gp::append(static_cast<gp::slice<int64_t>&&>(gg[k]), k);"},
		{"var k int\n\tgg[k] = append(gg[k], f())", "{ auto gp_t0 = f(); gg[k] = gp::append(static_cast<gp::slice<int64_t>&&>(gg[k]), gp_t0); }"},
		{"gg[f()] = append(gg[0], x)", "{ auto gp_t0 = f(); auto gp_t1 = gp::append(gg[0], x); gg[gp_t0] = static_cast<gp::slice<int64_t>&&>(gp_t1); }"},
		{"gg[0] = gg[0][1:]", "gg[0] = gp::sub(gg[0], 1);"},
		{"gg[0] = gg[0][1:f()]", "{ auto gp_t0 = f(); auto gp_t1 = gp::sub(gg[0], 1, gp_t0); gg[0] = static_cast<gp::slice<int64_t>&&>(gp_t1); }"},
		{"gg[0] = mk()", "{ auto gp_t0 = mk(); gg[0] = static_cast<gp::slice<int64_t>&&>(gp_t0); }"},
		{"var j, k int\n\tgg[k] = append(gg[k], gg[j][0])", "{ auto gp_t0 = gg[k]; gg[k] = gp::append(gp_t0, gg[j][0]); }"},
		{"ss := [][]int{{1}}\n\tss[0] = ss[0][1:]", "ss[0] = gp::sub(ss[0], 1);"},
		{"gsl = append(gsl, 1)", "gsl = gp::append(static_cast<gp::slice<int64_t>&&>(gsl), 1);"},
		{"gsl = append(gsl, f())", "{ auto gp_t0 = f(); gsl = gp::append(static_cast<gp::slice<int64_t>&&>(gsl), gp_t0); }"},
		{"gsl = append(gsl, x/2)", "gsl = gp::append(static_cast<gp::slice<int64_t>&&>(gsl), x / 2);"},
		{"z := 0\n\tgsl = append(gsl, x/z)", "gsl = gp::append(gsl, gp::div(x, z));"},
		{"ls := []int{1}\n\tp := &ls\n\tgsl = append(gsl, (*p)[0])", "gsl = gp::append(gsl, (*p)[0]);"},
		{"ss := [][]int{{1}}\n\tss[0] = append(ss[0], 2)", "ss[0] = gp::append(static_cast<gp::slice<int64_t>&&>(ss[0]), 2);"},
		{"ss := [][]int{{1}}\n\ti := 0\n\tss[i] = append(ss[0], 2)", "ss[i] = gp::append(ss[0], 2);"},
		{"ls := []int{1}\n\tp := &ls\n\t*p = append(*p, 2)", "*p = gp::append(static_cast<gp::slice<int64_t>&&>(*p), 2);"},
		{"ls := []int{1}\n\tp := &ls[0]\n\t_ = p", "gp::ptr<int64_t> p = gp::ptr<int64_t>(ls.gp_owner(), &ls[0]);"},
		{"p := &gsl[f()]\n\t_ = p", "auto gp_t0 = f(); gp::ptr<int64_t> p = gp::ptr<int64_t>(gsl.gp_owner(), &gsl[gp_t0]);"},
		{"ss := [][]int{{1}}\n\tp := &ss[0][0]\n\t_ = p", "gp::ptr<int64_t> p = gp::elem_ptr(ss[0], 0);"},
		// Arrays (step 3.6): an element of a package array, or through a pointer, is read after
		// the calls; one of a local array only panics (a variable index), and isn't copied as an
		// argument of fmt (it is addressable); len of an array is a constant unless it has a
		// call; the operands of a target a[i] are saved, never the array itself; &a[i] and a[:]
		// share the owner; range copies the array only when the body could change it.
		{"fmt.Println(ga[0], f())", "{ auto gp_t0 = f(); gp::fmt::Println(ga[0], gp_t0); }"},
		{"la := [2]int{1, 2}\n\tfmt.Println(la[0], f())", "gp::fmt::Println(la[0], f());"},
		{"la := [2]int{1, 2}\n\ti := 0\n\tfmt.Println(la[i], f())", "{ auto gp_t0 = f(); gp::fmt::Println(la[i], gp_t0); }"},
		{"la := [2]bool{}\n\tfmt.Println(la[0], f())", "gp::fmt::Println(la[0], f());"},
		{"p := &ga\n\tfmt.Println(p[1], f())", "{ auto gp_t0 = f(); gp::fmt::Println((*p)[1], gp_t0); }"},
		{"fmt.Println(len(ga), f())", "gp::fmt::Println(int64_t(3), f());"},
		{"fmt.Println(len(mka()), f())", "{ auto gp_t0 = gp::len(mka()); gp::fmt::Println(gp_t0, f()); }"},
		{"fmt.Println(ga == [3]int{}, f())", "{ auto gp_t0 = ga == gp::array<int64_t, 3>{}; gp::fmt::Println(gp_t0, f()); }"},
		{"la := [2]int{f(), f()}\n\t_ = la", "gp::array<int64_t, 2> la{f(), f()};"},
		{"la := [2]int{x, f()}\n\t_ = la", "auto gp_t0 = f(); gp::array<int64_t, 2> la{x, gp_t0};"},
		{"la := [3]int{1: f(), 0: f()}\n\t_ = la", "gp::array<int64_t, 3> la = gp::array_keyed<int64_t, 3>({{1, f()}, {0, f()}});"},
		{"lg := [2][2]int{{f()}, {1, f()}}\n\t_ = lg", "gp::array<gp::array<int64_t, 2>, 2> lg{gp::array<int64_t, 2>{f()}, gp::array<int64_t, 2>{1, f()}};"},
		{"ga[f()] += 1", "{ auto gp_t0 = f(); ga[gp_t0] = ga[gp_t0] + 1; }"},
		{"ga, ga[0] = [3]int{}, 5", "ga = gp::array<int64_t, 3>{}; ga[0] = 5;"},
		{"i := 0\n\ti, ga[i] = 1, 2", "{ auto gp_t0 = i; i = 1; ga[gp_t0] = 2; }"},
		{"j := 0\n\tj, gg2[j][1] = 1, 5", "{ auto gp_t0 = j; j = 1; gg2[gp_t0][1] = 5; }"},
		{"p, q := &ga, &ga\n\tp, p[0] = q, 1", "{ auto gp_t0 = p; p = q; (*gp_t0)[0] = 1; }"},
		{"ls := [][2]int{{1}}\n\tj := 0\n\tls, ls[j][1] = nil, 5", "{ auto gp_t0 = ls; ls = nullptr; gp_t0[j][1] = 5; }"},
		{"p := &ga[f()]\n\t_ = p", "auto gp_t0 = f(); gp::ptr<int64_t> p = gp::static_ptr(ga[gp_t0]);"},
		{"la := [2]int{}\n\tp := &la[1]\n\t_ = p", "gp::ptr<int64_t> p = gp::ptr<int64_t>(la, &(*la)[1]);"},
		{"pa := &ga\n\tp := &pa[2]\n\t_ = p", "gp::ptr<int64_t> p = gp::ptr<int64_t>(pa, &(*pa)[2]);"},
		{"p := &mka()[1]\n\t_ = p", "gp::ptr<int64_t> p = gp::elem_ptr(mka(), 1);"},
		{"ls := [][2]int{{1}}\n\tp := &ls[0][1]\n\t_ = p", "gp::ptr<int64_t> p = gp::ptr<int64_t>(ls.gp_owner(), &ls[0][1]);"},
		{"s := ga[1:]\n\t_ = s", "gp::slice<int64_t> s = gp::sub(gp::static_ptr(ga), 1);"},
		{"la := [3]int{}\n\ts := la[:2:3]\n\t_ = s", "gp::slice<int64_t> s = gp::sub(la, 0, 2, 3);"},
		{"s := mka()[:]\n\t_ = s", "gp::slice<int64_t> s = gp::sub(mka(), 0);"},
		{"s := gg2[1][:]\n\t_ = s", "gp::slice<int64_t> s = gp::sub(gp::static_ptr(gg2[1]), 0);"},
		{"la := [2]int(gsl)\n\t_ = la", "gp::array<int64_t, 2> la = gp::to_array<2>(gsl);"},
		{"fmt.Println([2]int(gsl)[0], f())", "{ auto gp_t0 = f(); gp::fmt::Println(gp::to_array<2>(gsl)[0], gp_t0); }"},
		{"p := (*[2]int)(gsl)\n\t_ = p", "gp::ptr<gp::array<int64_t, 2>> p = gp::to_array_ptr<2>(gsl);"},
		{"for i, v := range ga {\n\t\t_, _ = i, v\n\t}", "for (auto gp_it0 = gp::range_array(ga); gp::range_next(gp_it0);) {"},
		{"for _, v := range ga {\n\t\tga[0] = v\n\t}", "for (auto gp_it0 = gp::range_array_copy(ga); gp::range_next(gp_it0);) {"},
		{"for _, v := range ga {\n\t\t_ = v + f()\n\t}", "for (auto gp_it0 = gp::range_array_copy(ga); gp::range_next(gp_it0);) {"},
		{"la := [2]int{}\n\tfor _, v := range la {\n\t\t_ = v + f()\n\t}", "for (auto gp_it0 = gp::range_array(la); gp::range_next(gp_it0);) {"},
		{"la := [2]int{}\n\tfor _, v := range la {\n\t\tla[1] = v\n\t}", "for (auto gp_it0 = gp::range_array_copy(la); gp::range_next(gp_it0);) {"},
		{"lg := [2][2]int{}\n\tfor _, v := range lg[0] {\n\t\tlg[1][0] = v\n\t}", "for (auto gp_it0 = gp::range_array_copy(lg[0]); gp::range_next(gp_it0);) {"},
		{"la := [2]int{}\n\tfor i, v := range la {\n\t\ti, v = 1, 2\n\t\t_, _ = i, v\n\t}", "for (auto gp_it0 = gp::range_array(la); gp::range_next(gp_it0);) {"},
		{"la := [2]int{}\n\tvar i int\n\tfor i, la[1] = range la {\n\t}\n\t_ = i", "for (auto gp_it0 = gp::range_array_copy(la); gp::range_next(gp_it0);) {"},
		{"var t [1]int\n\tvar i int\n\tfor i, t[f()] = range ga {\n\t}\n\t_ = i", "for (auto gp_it0 = gp::range_array_copy(ga); gp::range_next(gp_it0);) {"},
		{"la := [2]int{}\n\tvar t [1]int\n\tvar i int\n\tfor i, t[f()] = range la {\n\t}\n\t_ = i", "for (auto gp_it0 = gp::range_array(la); gp::range_next(gp_it0);) {"},
		{"for _, v := range [2]int{f(), 1} {\n\t\t_ = v\n\t}", "for (auto gp_it0 = gp::range_array_copy(gp::array<int64_t, 2>{f(), 1}); gp::range_next(gp_it0);) {"},
		{"p := &ga\n\tfor _, v := range p {\n\t\tp[0] = v\n\t}", "for (auto gp_it0 = gp::range_array(p); gp::range_next(gp_it0);) {"},
		{"p, n := &ga, 0\n\tfor _, v := range *p {\n\t\tn += v\n\t}\n\t_ = n", "for (auto gp_it0 = gp::range_array(*p); gp::range_next(gp_it0);) {"},
		{"p := &ga\n\tfor _, v := range *p {\n\t\tp = nil\n\t\t_ = v\n\t}", "for (auto gp_it0 = gp::range_array_copy(*p); gp::range_next(gp_it0);) {"},
		{"ls := [][2]int{{1}}\n\tfor _, v := range ls[0] {\n\t\tls = nil\n\t\t_ = v\n\t}", "for (auto gp_it0 = gp::range_array_copy(ls[0]); gp::range_next(gp_it0);) {"},
		{"n := 0\n\tfor _, v := range *mka() {\n\t\tn += v\n\t}\n\t_ = n", "for (auto gp_it0 = gp::range_array_copy(*mka()); gp::range_next(gp_it0);) {"},
		{"n := 0\n\tfor _, v := range gg2[0] {\n\t\tn += v\n\t}\n\t_ = n", "for (auto gp_it0 = gp::range_array(gg2[0]); gp::range_next(gp_it0);) {"},
		{"big := [1 << 15]int{3: f()}\n\t_ = big", "gp::ptr<gp::array<int64_t, 32768>> big = gp::new_array_keyed<int64_t, 32768>({{3, f()}});"},
		{"big := [1 << 15]int{x, 2}\n\t_ = big", "gp::ptr<gp::array<int64_t, 32768>> big = gp::new_array<int64_t, 32768>({x, 2});"},
		{"p := &[1 << 15]int{3: x}\n\tq := &[1 << 15]int{}\n\t_, _ = p, q", "gp::ptr<gp::array<int64_t, 32768>> p = gp::new_array_keyed<int64_t, 32768>({{3, x}});"},
		{"p := &[1 << 15]int{}\n\t_ = p", "gp::ptr<gp::array<int64_t, 32768>> p = gp::new_<gp::array<int64_t, 32768>>();"},
		{"big := [1 << 15]int{x}\n\t{\n\t\tbig := [1 << 15]int{big[0]}\n\t\t_ = big\n\t}\n\t_ = big", "gp::ptr<gp::array<int64_t, 32768>> gp_t0 = gp::new_array<int64_t, 32768>({(*big)[0]}); { gp::ptr<gp::array<int64_t, 32768>> big = static_cast<gp::ptr<gp::array<int64_t, 32768>>&&>(gp_t0);"},
		{"big := [1 << 15]int(gsl)\n\t_ = big", "gp::ptr<gp::array<int64_t, 32768>> big = gp::new_to_array<32768>(gsl);"},
		{"la := [2]int{x, 2}\n\t_ = la", "gp::array<int64_t, 2> la{x, 2};"},
		{"for i := range mka() {\n\t\t_ = i\n\t}", "for (int64_t gp_i0 = 0, gp_n0 = gp::len(mka()); gp_i0 < gp_n0; gp_i0++) {"},
		{"var p *[3]int\n\tfor i := range *p {\n\t\t_ = i\n\t}", "for (int64_t gp_i0 = 0; gp_i0 < 3; gp_i0++) {"},
		{"var p *[3]int\n\tfor i, _ := range *p {\n\t\t_ = i\n\t}", "for (int64_t gp_i0 = 0, gp_n0 = gp::len(*p); gp_i0 < gp_n0; gp_i0++) {"},
		// Maps (step 3.7): m[k] is read after the calls, also of a local map (another variable
		// can share its table), but a bool is copied in its place as an argument of fmt, and so
		// is m[string(b)] (gc reads it there); len(m) is evaluated in its place; a store
		// evaluates its value first; op= and ++ take the entry once (gp::ref), but /= and %= by
		// a variable store a new value (the division panics before the entry is taken), and <<=
		// by a variable takes it twice; m[k] = append(m[k], x) takes the entry once, after the
		// other arguments, and moves the slice; v, ok := m[k] is gp::lookup; a map literal is
		// evaluated entry by entry, in a temporary when an entry needs one.
		{"fmt.Println(gm[1], f())", "{ auto gp_t0 = f(); gp::fmt::Println(gp::get(gm, 1), gp_t0); }"},
		{"lm := map[int]int{}\n\tfmt.Println(lm[1], x)", "gp::fmt::Println(gp::get(lm, 1), x);"},
		{"lm := map[int]int{}\n\tfmt.Println(lm[1], f())", "{ auto gp_t0 = f(); gp::fmt::Println(gp::get(lm, 1), gp_t0); }"},
		{"lm := map[int]bool{}\n\tfmt.Println(lm[1], f())", "{ auto gp_t0 = gp::get(lm, 1); gp::fmt::Println(gp_t0, f()); }"},
		{"b := []byte(\"x\")\n\tlm := map[string]int{}\n\tfmt.Println(lm[string(b)], f())", "{ auto gp_t0 = gp::get(lm, gp::to_string(b)); gp::fmt::Println(gp_t0, f()); }"},
		{"lm := map[int]int{}\n\tfmt.Println(len(lm), f())", "{ auto gp_t0 = gp::len(lm); gp::fmt::Println(gp_t0, f()); }"},
		{"gm[1] = f()", "{ auto gp_t0 = f(); gp::set(gm, 1, gp_t0); }"},
		{"lm := map[int]int{}\n\tlm[f()] = f()", "{ auto gp_t0 = f(); gp::set(lm, gp_t0, f()); }"},
		{"lm := map[int]int{}\n\tlm[1] = x", "gp::set(lm, 1, x);"},
		{"lm := map[int]int{}\n\tlm[1] += f()", "{ auto gp_t0 = f(); gp::ref(lm, 1) += gp_t0; }"},
		{"lm := map[int]int{}\n\tlm[f()]++", "gp::ref(lm, f())++;"},
		{"lm := map[int]int{}\n\tlm[f()] -= 2", "{ auto gp_t0 = f(); gp::ref(lm, gp_t0) -= 2; }"},
		{"lm := map[int]int{}\n\tz := 0\n\tlm[1] /= z", "gp::set(lm, 1, gp::div(gp::get(lm, 1), z));"},
		{"lm := map[int]int{}\n\tlm[1] /= -1", "gp::set(lm, 1, gp::div(gp::get(lm, 1), -1));"},
		{"lm := map[int]int{}\n\tlm[1] %= 3", "gp::ref(lm, 1) %= 3;"},
		{"lm := map[int]float64{}\n\tz := 0.0\n\tlm[1] /= z", "gp::ref(lm, 1) /= z;"},
		{"lm := map[int]int{}\n\tn := 1\n\tlm[1] <<= n", "gp::ref(lm, 1) = gp::shl(gp::ref(lm, 1), n);"},
		{"lm := map[int]int{}\n\tlm[1] >>= x", "{ auto gp_t0 = x; gp::ref(lm, 1) = gp::shr(gp::ref(lm, 1), gp_t0); }"},
		{"lm := map[int]uint8{}\n\tlm[1] <<= 7", "gp::ref(lm, 1) <<= 7;"},
		{"lm := map[int]int{}\n\tlm[1] &^= x", "gp::ref(lm, 1) &= ~x;"},
		{"lm := map[int][]int{}\n\tk := 1\n\tlm[k] = append(lm[k], 2)", "{ gp::slice<int64_t>& gp_r0 = gp::ref(lm, k); gp_r0 = gp::append(static_cast<gp::slice<int64_t>&&>(gp_r0), 2); }"},
		{"gms[1] = append(gms[1], f(), x)", "{ auto gp_t0 = f(); auto gp_t1 = x; gp::slice<int64_t>& gp_r0 = gp::ref(gms, 1); gp_r0 = gp::append(static_cast<gp::slice<int64_t>&&>(gp_r0), gp_t0, gp_t1); }"},
		{"lm := map[int][]int{}\n\tlm[1] = append(lm[2], 1)", "gp::set(lm, 1, gp::append(gp::get(lm, 2), 1));"},
		{"lm := map[int][]int{}\n\tp := &lm\n\tk := 1\n\t(*p)[k+1] = append((*p)[k+1], f())", "{ auto gp_t0 = *p; auto gp_t1 = f(); gp::slice<int64_t>& gp_r0 = gp::ref(gp_t0, k + 1); gp_r0 = gp::append(static_cast<gp::slice<int64_t>&&>(gp_r0), gp_t1); }"},
		{"lm := map[int]int{}\n\tv, ok := lm[1]\n\t_, _ = v, ok", "auto gp_t0 = gp::lookup(lm, 1);"},
		{"lm := map[int]int{}\n\tlm[1], lm[2] = lm[2], lm[1]", "{ auto gp_t0 = gp::get(lm, 1); gp::set(lm, 1, gp::get(lm, 2)); gp::set(lm, 2, gp_t0); }"},
		{"lm := map[int]int{}\n\tlm, lm[1] = nil, 2", "{ auto gp_t0 = lm; lm = nullptr; gp::set(gp_t0, 1, 2); }"},
		{"lm := map[int]int{1: 2, 3: f()}\n\t_ = lm", "gp::map<int64_t, int64_t> lm = gp::map_of<int64_t, int64_t>({{1, 2}, {3, f()}});"},
		{"lm := map[int]int{3: f(), 1: 2}\n\t_ = lm", "gp::map<int64_t, int64_t> lm = gp::map_of<int64_t, int64_t>({{1, 2}, {3, f()}});"},
		{"lm := map[int]int{f(): x, 2: f()}\n\t_ = lm", "gp::map<int64_t, int64_t> lm = gp::map_of<int64_t, int64_t>({{f(), x}, {2, f()}});"},
		{"lm := map[int]int{1: 2, x: f()}\n\t_ = lm", "gp::map<int64_t, int64_t> gp_t0 = gp::map_of<int64_t, int64_t>({{1, 2}}); auto gp_t1 = f(); gp::set(gp_t0, x, gp_t1); gp::map<int64_t, int64_t> lm = gp_t0;"},
		{"fmt.Println(len(map[int]int{1: x}), f())", "{ auto gp_t0 = gp::len(gp::map_of<int64_t, int64_t>({{1, x}})); gp::fmt::Println(gp_t0, f()); }"},
		{"for k, v := range gm {\n\t\t_, _ = k, v\n\t}", "for (auto gp_it0 = gp::range_map(gm); gp::range_next(gp_it0);) {"},
		{"delete(gm, f())", "{ auto gp_t0 = f(); gp::erase(gm, gp_t0); }"},
		{"lm := make(map[int]int, x)\n\t_ = lm", "gp::map<int64_t, int64_t> lm = gp::make_map<int64_t, int64_t>(x);"},
		// Structs (step 3.8): a field of a package struct, or through a pointer, is read after
		// the calls, one of a local struct anywhere; a keyed literal is in the fields' order, with
		// temporaries where Go's source order differs; the pointer of a target p.f is saved; &x.f
		// shares the owner (gp::field_ptr for a call); an assignment of a struct that holds owners
		// from a place behind a pointer or a slice copies it first; s.f = append(s.f, x) moves;
		// a struct argument of fmt is copied by gc's rule (in Go's sizes).
		{"fmt.Println(gst.a, f())", "{ auto gp_t0 = f(); gp::fmt::Println(gst.a, gp_t0); }"},
		{"ls := Pt{}\n\tfmt.Println(ls.a, f())", "gp::fmt::Println(ls.a, f());"},
		{"lp := &gst\n\tfmt.Println(lp.a, f())", "{ auto gp_t0 = f(); gp::fmt::Println(lp->a, gp_t0); }"},
		{"gst.a = f()", "gst.a = f();"},
		{"gsts[0].b = f()", "{ auto gp_t0 = f(); gsts[0].b = gp_t0; }"},
		{"lp := &gst\n\tlp.a += f()", "{ auto gp_t0 = f(); lp->a = lp->a + gp_t0; }"},
		{"mkp().b += 1", "{ auto gp_t0 = mkp(); gp_t0->b = gp_t0->b + 1; }"},
		{"ls := Pt{b: f(), a: f()}\n\t_ = ls", "auto gp_t0 = f(); Pt ls{.a = f(), .b = gp_t0};"},
		{"ls := Pt{a: f(), b: f()}\n\t_ = ls", "Pt ls{.a = f(), .b = f()};"},
		{"ls := Pt{f(), f()}\n\t_ = ls", "Pt ls{f(), f()};"},
		{"ls := Pt{b: x, a: f()}\n\t_ = ls", "Pt ls{.a = f(), .b = x};"},
		{"ls := []Pt{{1, 2}, {b: 3}}\n\t_ = ls", "gp::slice<Pt> ls = gp::slice_of<Pt>({Pt{1, 2}, Pt{.b = 3}});"},
		{"lps := []*Pt{{1, 2}}\n\t_ = lps", "gp::slice<gp::ptr<Pt>> lps = gp::slice_of<gp::ptr<Pt>>({gp::new_<Pt>(Pt{1, 2})});"},
		{"lp, lq := &gst, &Pt{}\n\tlp, lp.a = lq, 1", "{ auto gp_t0 = lp; lp = lq; gp_t0->a = 1; }"},
		{"i := 0\n\ti, gsts[i].a = 1, 2", "{ auto gp_t0 = i; i = 1; gsts[gp_t0].a = 2; }"},
		{"lp := &gst\n\tq := &lp.b\n\t_ = q", "gp::ptr<int64_t> q = gp::ptr<int64_t>(lp, &lp->b);"},
		{"q := &gst.b\n\t_ = q", "gp::ptr<int64_t> q = gp::static_ptr(gst.b);"},
		{"q := &gsts[0].b\n\t_ = q", "gp::ptr<int64_t> q = gp::ptr<int64_t>(gsts.gp_owner(), &gsts[0].b);"},
		{"q := &mkp().b\n\t_ = q", "gp::ptr<int64_t> q = gp::field_ptr(mkp(), &Pt::b);"},
		{"ls := Pt{}\n\tq := &ls.a\n\t_ = q", "gp::ptr<int64_t> q = gp::ptr<int64_t>(ls, &(*ls).a);"},
		{"lt := Tree{}\n\tlt = lt.kids[0]", "lt = Tree(lt.kids[0]);"},
		{"lt := Tree{}\n\tlp := &lt\n\t*lp = *lt.next", "*lp = Tree(*(*lt).next);"},
		{"lp := &gst\n\tgst = *lp", "gst = *lp;"},
		{"lt := Tree{}\n\tlt.kids = append(lt.kids, Tree{})", "lt.kids = gp::append(static_cast<gp::slice<Tree>&&>(lt.kids), Tree{});"},
		{"fmt.Println(Pt{x, 1}, f())", "{ auto gp_t0 = Pt{x, 1}; gp::fmt::Println(gp::fmt::as(&gp_td_Pt, gp_t0), f()); }"},
		{"fmt.Println(One{x}, f())", "{ auto gp_t0 = f(); gp::fmt::Println(gp::fmt::as(&gp_td_One, One{x}), gp_t0); }"},
		{"fmt.Println(gst == Pt{}, f())", "{ auto gp_t0 = gst == Pt{}; gp::fmt::Println(gp_t0, f()); }"},
		// A target that is a part of a plain local (w.a, la[0], w.p, ws[0].p) is a write to
		// it: a later target's operand that reads the local is saved first (Go's two phases).
		{"var w Pt\n\tls := []int{0, 0}\n\tw.a, ls[w.a] = 1, 3", "{ auto gp_t0 = w.a; w.a = 1; ls[gp_t0] = 3; }"},
		{"la := [2]int{}\n\tlx := [3]int{}\n\tla[0], lx[la[0]] = 1, 2", "{ auto gp_t0 = la[0]; la[0] = 1; lx[gp_t0] = 2; }"},
		{"var w Tree\n\tw.next, w.next.next = &Tree{}, nil", "{ auto gp_t0 = w.next; w.next = gp::new_<Tree>(Tree{}); gp_t0->next = nullptr; }"},
		{"ws := [1]Tree{}\n\tws[0].next, ws[0].next.next = nil, nil", "{ auto gp_t0 = ws[0].next; ws[0].next = nullptr; gp_t0->next = nullptr; }"},
		{"var w Pt\n\tls := []int{0, 0}\n\tfor w.a, ls[w.a] = range ls {\n\t}", "auto gp_t0 = w.a; w.a = gp_it0.gp_i; ls[gp_t0] = gp_it0.gp_v();"},
		{"var w Pt\n\tw.a, w.b = w.b, w.a", "{ auto gp_t0 = w.a; w.a = w.b; w.b = gp_t0; }"},
		// Embedded pointers: the pointer of a promoted field is saved like p of p.f.
		{"var w Wrap\n\tw.Pt, w.a = &Pt{}, 1", "{ auto gp_t0 = w.Pt; w.Pt = gp::new_<Pt>(Pt{}); gp_t0->a = 1; }"},
		// A struct over 128 KiB is made in its box, then its fields are stored.
		{"lb := Huge{n: f()}\n\t_ = lb", "gp::ptr<Huge> lb = gp::new_init<Huge>([&](Huge& gp_b) { gp_b.n = f(); });"},
		{"lb := &Huge{n: x, v: f()}\n\t_ = lb", "gp::ptr<Huge> lb = gp::new_init<Huge>([&](Huge& gp_b) { gp_b.v = f(); gp_b.n = x; });"},
		{"lb := Huge{n: f(), v: f()}\n\t_ = lb", "auto gp_t0 = f(); gp::ptr<Huge> lb = gp::new_init<Huge>([&](Huge& gp_b) { gp_b.v = f(); gp_b.n = gp_t0; });"},
		// Struct keys and values of maps (with step 3.7): a struct literal of constants is a
		// static entry, stored first; a field of a value read from a map is read from the copy.
		{"lmk := map[Pt]int{}\n\tlmk[Pt{1, 2}] = f()", "gp::set(lmk, Pt{1, 2}, f());"},
		{"lmk := map[Pt]int{{x, 1}: 1, {1, 2}: 2}\n\t_ = lmk", "gp::map<Pt, int64_t> lmk = gp::map_of<Pt, int64_t>({{Pt{1, 2}, 2}, {Pt{x, 1}, 1}});"},
		{"lmv := map[int]Pt{}\n\tfmt.Println(lmv[1].a, f())", "{ auto gp_t0 = f(); gp::fmt::Println(gp::get(lmv, 1).a, gp_t0); }"},
		// defer (step 3.13): the arguments are ordered like those of the call as a statement and
		// stored; a function of the package is stored as a function pointer, anything else is
		// called by a generic lambda, with each argument that is not a constant its parameter.
		{"defer g(1, f())", "gp_df.gp_push(g, 1, f());"},
		{"defer g(x, f())", "{ auto gp_t0 = f(); gp_df.gp_push(g, x, gp_t0); }"},
		{"l := 1\n\tdefer g(f(), l)", "gp_df.gp_push(g, f(), l);"},
		{"defer g(f(), f())", "{ auto gp_t0 = f(); gp_df.gp_push(g, gp_t0, f()); }"},
		{"defer fmt.Println(x, f())", "{ auto gp_t0 = f(); gp_df.gp_push_call([](auto&& gp_a0, auto&& gp_a1) { gp::fmt::Println(gp_a0, gp_a1); }, x, gp_t0); }"},
		{"defer fmt.Println(\"x\", 1)", "gp_df.gp_push_call([](auto&&...) { gp::fmt::Println(\"x\"_gs, int64_t(1)); });"},
		{"defer println(x == 2, f())", "{ auto gp_t0 = f(); gp_df.gp_push_call([](auto&& gp_a0, auto&& gp_a1) { gp::println(gp_a0, gp_a1); }, x == 2, gp_t0); }"},
		{"p := &x\n\tdefer g(*p, f())", "{ auto gp_t0 = f(); gp_df.gp_push(g, *p, gp_t0); }"},
		{"defer g(gsl[0], f())", "{ auto gp_t0 = f(); gp_df.gp_push(g, gsl[0], gp_t0); }"},
		{"defer delete(gm, f())", "{ auto gp_t0 = f(); gp_df.gp_push_call([](auto&& gp_a0, auto&& gp_a1) { gp::erase(gp_a0, gp_a1); }, gm, gp_t0); }"},
		{"defer panic(f())", "gp_df.gp_push_call([](auto&& gp_a0) { gp::panic(gp_a0); }, f());"},
		{"defer fmt.Println(two())", "gp_df.gp_push_call([](auto&& gp_a0) { std::apply([](auto&& gp_r0, auto&& gp_r1) { return gp::fmt::Println(gp_r0, gp_r1); }, gp_a0); }, two());"},
		// Func values (step 3.11): the func value is in its call's scope, first; a local that
		// isn't boxed is called in place, a boxed one is read after the calls (gp_call); a call
		// that gives the func goes first; variadic arguments are one slice.
		{"fv := func(a int) int { return a }\n\tfmt.Println(fv(1), f())", "{ auto gp_t0 = fv(1); gp::fmt::Println(gp_t0, f()); }"},
		{"fv := func(a int) int { return a }\n\tfmt.Println(fv(f()))", "gp::fmt::Println(fv(f()));"},
		{"bv := func(a int) int { return a }\n\t_ = &bv\n\tfmt.Println(bv(f()))", "{ auto gp_t0 = f(); gp::fmt::Println((*bv).gp_call(gp_t0)); }"},
		{"mk := func() func(int) int { return nil }\n\t_ = mk()(f())", "{ auto gp_t0 = mk(); static_cast<void>(gp_t0(f())); }"},
		{"sum := func(xs ...int) int { return len(xs) }\n\tfmt.Println(sum(f(), x))", "{ auto gp_t0 = f(); gp::fmt::Println(sum(gp::slice_of<int64_t>({gp_t0, x}))); }"},
		{"fv := func(a int) int { return a }\n\tdefer fv(f())", "gp_df.gp_push_call(fv, f());"},
		{"defer g(two())", "gp_df.gp_push_call([](auto&& gp_a0) { std::apply(gpkg_main::g, gp_a0); }, two());"},
		// Named types (step 3.9): a package N is read after the calls, a local one anywhere; a
		// division of a named int is gp::div with its temporaries like an int's; a value that Go
		// converts to a named type implicitly is converted in its temporary; s = append(s, x)
		// of a named slice moves it; an && of a named bool is that type, also in a temporary.
		{"fmt.Println(gn, f())", "{ auto gp_t0 = f(); gp::fmt::Println(gp::fmt::as(&gp_td_N, gn), gp_t0); }"},
		{"ln := gn\n\tfmt.Println(ln*2, f())", "gp::fmt::Println(gp::fmt::as(&gp_td_N, ln * N(2)), f());"},
		{"z := N(0)\n\tfmt.Println(gn/z + N(f()))", "{ auto gp_t0 = f(); gp::fmt::Println(gp::fmt::as(&gp_td_N, gp::div(gn, z) + N(gp_t0))); }"},
		{"var lns NS = mkn()\n\t_ = lns", "NS lns = NS(mkn());"},
		{"lns := NS{}\n\tlns = append(lns, gn)", "lns = NS(gp::append(static_cast<NS&&>(lns), gn));"},
		{"lns := NS{}\n\tlns = mkn()\n\t_ = lns", "lns = NS(mkn());"},
		{"var lns NS\n\tlns, x = mkn(), f()\n\t_ = lns", "{ auto gp_t0 = NS(mkn()); auto gp_t1 = f(); lns = gp_t0; x = gp_t1; }"},
		{"lb := Flag(x > 0) && Flag(f() > 1)\n\t_ = lb", "Flag lb = Flag(Flag(x > 0) && Flag(f() > 1));"},
		{"fmt.Println(Flag(x > 0) && Flag(f() > 1), f())", "{ Flag gp_t0 = Flag(Flag(x > 0) && Flag(f() > 1)); gp::fmt::Println(gp::fmt::as(&gp_td_Flag, gp_t0), f()); }"},
		{"switch gn {\n\tcase 1:\n\t}", "switch (gn.gp_value()) {"},
		// Methods (step 3.10): a receiver whose address is taken has no read of its own, a call
		// in the receiver goes first, a value method through a package pointer copies it first
		// (the copy holds the object), through a local pointer it doesn't, a read of a package
		// receiver goes after the calls, and a deferred call stores its receiver.
		{"gst.inc(f())", "Pt::inc(gp::borrow(&gst), f());"},
		{"mkp().inc(f())", "{ auto gp_t0 = mkp(); Pt::inc(gp::lend(gp_t0), f()); }"},
		{"fmt.Println(gpp.sum())", "gp::fmt::Println(gp::ptr<Pt>(gpp)->sum());"},
		{"lp := &gst\n\tfmt.Println(lp.sum())", "gp::fmt::Println(lp->sum());"},
		{"lp := &gst\n\t_ = lp.sum() + f()", "{ auto gp_t0 = lp->sum(); static_cast<void>(gp_t0 + f()); }"},
		{"fmt.Println(gpp.inc(f()))", "{ auto gp_t0 = f(); gp::fmt::Println(Pt::inc(gp::lend(gpp), gp_t0)); }"},
		{"fmt.Println(gst.sum(), f())", "{ auto gp_t0 = gst.sum(); gp::fmt::Println(gp_t0, f()); }"},
		{"gst.add(f())", "{ auto gp_t0 = f(); gst.add(gp_t0); }"},
		{"gsts[0].inc(1)", "Pt::inc(gp::borrow(&gsts[0]), 1);"},
		{"defer gpp.sum()", "gp_df.gp_push_call([](auto&& gp_a0) { gp_a0.sum(); }, *gpp);"},
		{"defer gst.inc(f())", "gp_df.gp_push_call([](auto&& gp_a0, auto&& gp_a1) { Pt::inc(gp::lend(gp_a0), gp_a1); }, gp::static_ptr(gst), f());"},
		// Method values (step 3.10b): a copy of the receiver (*gpp for a value method through a
		// pointer), or the pointer, in the lambda's capture, read after the calls of the scope;
		// a call of a method expression calls its lambda.
		{"s := gst.sum\n\t_ = s", "gp::func<int64_t()> s = gp::func<int64_t()>([gp_r = gst](auto&&... gp_a) { return gp_r.sum(gp_a...); });"},
		{"a := gpp.add\n\t_ = a", "gp::func<int64_t(int64_t)> a = gp::func<int64_t(int64_t)>([gp_r = *gpp](auto&&... gp_a) { return gp_r.add(gp_a...); });"},
		{"i := gst.inc\n\t_ = i", "gp::func<int64_t(int64_t)> i = gp::func<int64_t(int64_t)>([gp_r = gp::static_ptr(gst)](auto&& gp_a0) { return Pt::inc(gp::lend(gp_r), gp_a0); });"},
		{"fs := []func() int{gst.sum, mkf()}\n\t_ = fs", "auto gp_t0 = mkf(); gp::slice<gp::func<int64_t()>> fs = gp::slice_of<gp::func<int64_t()>>({gp::func<int64_t()>([gp_r = gst](auto&&... gp_a) { return gp_r.sum(gp_a...); }), gp_t0});"},
		{"_ = Pt.add(gst, f())", "{ auto gp_t0 = f(); static_cast<void>([](auto&& gp_r, auto&&... gp_a) { return gp_r.add(gp_a...); }(gst, gp_t0)); }"},
		// Two operations that can panic (step 3.12): the first goes into a temporary when C++
		// could evaluate the other first; a place (the target of op=, the operand of &) or a
		// struct is only checked first; none is needed in a list that C++ evaluates in order;
		// a map store's value goes first; with several targets, gc's ascompatee order.
		{"i := 1\n\tfmt.Println(gsl[i], ga[i])", "{ auto gp_t0 = gsl[i]; gp::fmt::Println(gp_t0, ga[i]); }"},
		{"i := 1\n\t_ = gsl[i] + ga[i]*2", "{ auto gp_t0 = gsl[i]; static_cast<void>(gp_t0 + ga[i] * 2); }"},
		{"i := 1\n\t[]int{1}[i] += ga[i]", "{ static_cast<void>(gp::slice_of<int64_t>({1})[i]); gp::slice_of<int64_t>({1})[i] = gp::slice_of<int64_t>({1})[i] + ga[i]; }"},
		{"i := 1\n\tdefer fmt.Println(gsl[i], ga[i])", "{ auto gp_t0 = gsl[i]; gp_df.gp_push_call([](auto&& gp_a0, auto&& gp_a1) { gp::fmt::Println(gp_a0, gp_a1); }, gp_t0, ga[i]); }"},
		{"i := 1\n\tgsl[i] += ga[i]", "{ auto& gp_r0 = gsl[i]; gp_r0 = gp_r0 + ga[i]; }"},
		{"i := 1\n\tfmt.Println(&gsl[i] == &ga[i])", "{ static_cast<void>(gsl[i]); gp::fmt::Println(gp::ptr<int64_t>(gsl.gp_owner(), &gsl[i]) == gp::static_ptr(ga[i])); }"},
		{"i := 1\n\tfmt.Println(gsts[i], ga[i])", "{ static_cast<void>(gsts[i]); gp::fmt::Println(gp::fmt::as(&gp_td_Pt, gsts[i]), ga[i]); }"},
		{"i := 1\n\t_ = []int{gsl[i], ga[i]}", "static_cast<void>(gp::slice_of<int64_t>({gsl[i], ga[i]}));"},
		{"i := 1\n\t_ = Pt{b: gsl[i], a: ga[i]}", "{ auto gp_t0 = gsl[i]; static_cast<void>(Pt{.a = ga[i], .b = gp_t0}); }"},
		{"i := 1\n\tgm[gsl[i]] = ga[i]", "{ auto gp_t0 = ga[i]; gp::set(gm, gsl[i], gp_t0); }"},
		{"i := 1\n\tgsl[ga[i]] = gsl[i]", "{ auto gp_t0 = gsl[i]; gsl[ga[i]] = gp_t0; }"},
		{"i := 1\n\ta := 0\n\tgm[gsl[i]], a = ga[i], 1\n\t_ = a", "{ auto& gp_r0 = gp::ref(gm, gsl[i]); gp_r0 = ga[i]; a = 1; }"},
		{"i := 1\n\tx, ga[0] = gsl[i], ga[i]", "{ auto gp_t0 = ga[i]; x = gsl[i]; ga[0] = gp_t0; }"},
		{"i := 1\n\ta, b := gsl[i], ga[i]\n\t_, _ = &a, b", "int64_t gp_t0 = ga[i]; gp::ptr<int64_t> a = gp::new_<int64_t>(gsl[i]); int64_t b = gp_t0;"},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			dir := t.TempDir()
			src := "package main\n\nimport \"fmt\"\n\nvar x = 1\n\nvar gsl = []int{1, 2}\n\nvar gg = [][]int{{1}}\n\nvar gm = map[int]int{}\n\nvar gms = map[int][]int{}\n\nfunc mk() []int {\n\treturn nil\n}\n\nfunc f() int {\n\tx++\n\tfmt.Print()\n\treturn x\n}\n\n" +
				"func g(a, b int) int {\n\treturn a + b\n}\n\nfunc h(p *int) *int {\n\treturn p\n}\n\nfunc two() (int, int) {\n\treturn 1, 2\n}\n\n" +
				"var ga = [3]int{1, 2, 3}\n\nvar gg2 = [2][2]int{}\n\nfunc mka() *[3]int {\n\treturn nil\n}\n\n" +
				"type Pt struct{ a, b int }\n\ntype One struct{ v int }\n\ntype Tree struct {\n\tkids []Tree\n\tnext *Tree\n}\n\nvar gst Pt\n\nvar gsts = []Pt{{1, 2}}\n\n" +
				"type Wrap struct{ *Pt }\n\ntype Huge struct {\n\tv  int\n\tbig [1 << 15]int\n\tn  int\n}\n\n" +
				"func mkp() *Pt {\n\treturn &gst\n}\n\n" + nt +
				"func (p Pt) sum() int {\n\treturn p.a + p.b\n}\n\nfunc (p Pt) add(n int) int {\n\treturn p.a + n\n}\n\n" +
				"func (p *Pt) inc(n int) int {\n\tp.a += n\n\treturn p.a\n}\n\nvar gpp = &gst\n\nfunc mkf() func() int {\n\tx++\n\treturn nil\n}\n\n" + "type Flag bool\n\nfunc main() {\n\t" + tt.body + "\n}\n"
			gp := filepath.Join(dir, "main.gp")
			if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			cpp, err := emitPatterns(t, Config{}, dir, gp)
			if err != nil {
				t.Fatal(err)
			}
			main := cpp[strings.Index(cpp, "void main() {"):]
			if !strings.Contains(main, "\n    "+tt.want+"\n") && !strings.Contains(main, " "+tt.want+"\n") {
				t.Errorf("the C++ has no line\n\t%s\n%s", tt.want, main)
			}
		})
	}
}

// TestOrderLarge checks that ordering a long expression stays fast (plan is O(n^2) in the
// items of a scope): 400 terms, with and without calls, in well under a second.
func TestOrderLarge(t *testing.T) {
	const n = 400
	var plain, calls []string
	for k := range n {
		plain = append(plain, "int(s[i])/i")
		if k%2 == 0 {
			calls = append(calls, "f()")
		} else {
			calls = append(calls, "int(s[i])/i+x")
		}
	}
	dir := t.TempDir()
	src := "package main\n\nvar x = 1\n\nfunc f() int {\n\tx++\n\treturn x\n}\n\nfunc main() {\n\ts, i := \"abc\", 1\n" +
		"\tprintln(" + strings.Join(plain, " + ") + ")\n" +
		"\tprintln(" + strings.Join(calls, ", ") + ")\n" +
		"\tprintln(" + strings.Join(calls, " + ") + ")\n}\n"
	gp := filepath.Join(dir, "main.gp")
	if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := emitPatterns(t, Config{}, dir, gp); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("loading, checking and emitting took %v, want less than a second", d)
	}
}
