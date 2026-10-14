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

// TestOrderPanics checks the order of calls and of the operations that can panic (order.go)
// against real Go: gc calls f first in a/b + f() and panics after it, and reads s after grow
// in int(s[i]) + grow(). Each program panics, with Go's output and exit code 2, on both
// compilers. (Which of two panicking operations without a call between them panics first is
// not kept: see order.go.)
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
			checkRunsNorm(t, emitter(gp), want, maskPC)
		})
	}
}

// TestOrderTemps checks where order.go puts temporaries, and where it doesn't: a call gets one
// only next to an operand that is not a constant or a local variable, and that C++ doesn't
// evaluate after it. (TestEmit runs the same cases in testdata/emit/order.gp against Go.)
func TestOrderTemps(t *testing.T) {
	tests := []struct{ body, want string }{
		// No temporaries.
		{"fmt.Println(f())", "gp::fmtshim::Println(f());"},
		{"l := 1\n\tg(l, f())", "g(l, f());"},
		{"g(1, f())", "g(1, f());"},
		{"l := 2\n\ty := f() + l*2\n\tfmt.Println(y)", "int64_t y = f() + l * 2;"},
		{"fmt.Println(g(f(), 1) / 3)", "gp::fmtshim::Println(g(f(), 1) / 3);"},
		{"z := 0\n\tfmt.Println(f() / z)", "gp::fmtshim::Println(gp::div(f(), z));"},
		{"if f() > 0 && f() < 9 {\n\t}", "if (f() > 0 && f() < 9) {"},
		{"for f() < 3 {\n\t}", "while (f() < 3) {"},
		{"a, b := f(), f()\n\tfmt.Println(a, b)", "int64_t a = f();"},
		{"fmt.Println(x)", "gp::fmtshim::Println(x);"},
		{"var b bool\n\tfmt.Println(b, f())", "gp::fmtshim::Println(b, f());"},
		// Temporaries.
		{"fmt.Println(f(), f())", "{ auto gp_t0 = f(); gp::fmtshim::Println(gp_t0, f()); }"},
		{"g(x, f())", "{ auto gp_t0 = f(); g(x, gp_t0); }"},
		{"g(f(), x)", "{ auto gp_t0 = f(); g(gp_t0, x); }"},
		{"y := x + f()\n\tfmt.Println(y)", "auto gp_t0 = f(); int64_t y = x + gp_t0;"},
		{"x += f()", "{ auto gp_t0 = f(); x = x + gp_t0; }"},
		{"a, z := 7, 0\n\tfmt.Println(a/z + f())", "{ auto gp_t0 = f(); gp::fmtshim::Println(gp::div(a, z) + gp_t0); }"},
		{"for x < f() {\n\t}", "auto gp_t0 = f(); if (!(x < gp_t0)) break;"},
		{"if f() > 0 && g(x, f()) > 0 {\n\t}", "bool gp_t0 = f() > 0; if (gp_t0) { auto gp_t1 = f(); gp_t0 = g(x, gp_t1) > 0; }"},
		{"s, i, j := \"ab\", 1, 0\n\tfmt.Println(s[i], s[j], s[i] == s[j])", "gp::fmtshim::Println(s[i], s[j], s[i] == s[j]);"},
		{"a, b := 7, 0\n\tfmt.Println(a/b, 1<<b, a%b)", "gp::fmtshim::Println(gp::div(a, b), gp::shl(int64_t(1), b), gp::rem(a, b));"},
		{"fmt.Println(x == 2, f())", "{ auto gp_t0 = x == 2; gp::fmtshim::Println(gp_t0, f()); }"},
		{"println(x == 2, f())", "{ auto gp_t0 = f(); gp::println(x == 2, gp_t0); }"},
		// Pointers (step 3.3): a boxed variable and *p are read after the calls, like x; a
		// local pointer itself is not.
		{"l := 1\n\t_ = &l\n\tfmt.Println(l, f())", "{ auto gp_t0 = f(); gp::fmtshim::Println((*l), gp_t0); }"},
		{"l := 1\n\t_ = &l\n\tfmt.Println(l == 1, f())", "{ auto gp_t0 = (*l) == 1; gp::fmtshim::Println(gp_t0, f()); }"},
		{"p := &x\n\tfmt.Println(*p, f())", "{ auto gp_t0 = f(); gp::fmtshim::Println(*p, gp_t0); }"},
		{"p := &x\n\tg(*p, 1)", "g(*p, 1);"},
		{"p := &x\n\t*p = f()", "*p = f();"},
		{"p := &x\n\t*p += f()", "{ auto gp_t0 = f(); *p = *p + gp_t0; }"},
		{"p := &x\n\tq := &p\n\t*p = f()\n\t_ = q", "{ auto gp_t0 = f(); *(*p) = gp_t0; }"},
		{"p := &x\n\t*h(p) += 1", "{ auto gp_t0 = gpkg_main::h(p); *gp_t0 = *gp_t0 + 1; }"},
		{"p := &x\n\t*h(p) = f()", "{ auto gp_t0 = gpkg_main::h(p); *gp_t0 = f(); }"},
		{"p := &x\n\t(*h(p))++", "(*gpkg_main::h(p))++;"},
		{"p, q := &x, new(int)\n\t*p, *q = *q, *p", "{ auto gp_t0 = *q; auto gp_t1 = *p; *p = gp_t0; *q = gp_t1; }"},
		{"p, q := &x, new(int)\n\tp, *p = q, 1", "{ auto gp_t0 = p; p = q; *gp_t0 = 1; }"},
		{"l := 1\n\tp := &l\n\tswitch l {\n\tcase f():\n\t}\n\t_ = p", "auto gp_tag0 = (*l);"},
		// Slices (step 3.5): an element is read after the calls (a call can change it through
		// another slice of its array), also one of a local slice and a byte, which is not
		// copied as an argument of fmt (it is addressable); len and cap of a package slice
		// are evaluated in their places; s = append(s, x) and s = s[a:b] move s when nothing
		// else in the call reads it.
		{"ls := []int{1}\n\tfmt.Println(ls[0], f())", "{ auto gp_t0 = f(); gp::fmtshim::Println(ls[0], gp_t0); }"},
		{"ls := []int{1}\n\tfmt.Println(ls[0], x)", "gp::fmtshim::Println(ls[0], x);"},
		{"b := []byte(\"x\")\n\tfmt.Println(b[0], f())", "{ auto gp_t0 = f(); gp::fmtshim::Println(b[0], gp_t0); }"},
		{"b := []byte(\"x\")\n\tfmt.Println(b[0] == 1, f())", "{ auto gp_t0 = b[0] == 1; gp::fmtshim::Println(gp_t0, f()); }"},
		{"b := []byte(\"x\")\n\tfmt.Println(string(b), f())", "{ auto gp_t0 = f(); gp::fmtshim::Println(gp::to_string(b), gp_t0); }"},
		{"str := \"x\"\n\tfmt.Println([]byte(str)[0], f())", "{ auto gp_t0 = f(); gp::fmtshim::Println(gp::to_bytes(str)[0], gp_t0); }"},
		{"fmt.Println(len(gsl), f())", "{ auto gp_t0 = gp::len(gsl); gp::fmtshim::Println(gp_t0, f()); }"},
		{"fmt.Println(cap(gsl) - len(gsl), f())", "{ auto gp_t0 = gp::cap(gsl); auto gp_t1 = gp::len(gsl); gp::fmtshim::Println(gp_t0 - gp_t1, f()); }"},
		{"ls := []int{1}\n\tfmt.Println(len(ls), f())", "gp::fmtshim::Println(gp::len(ls), f());"},
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
		{"ls := []int{1}\n\tls[0], ls[0] = ls[0], f()", "{ auto gp_t0 = f(); auto gp_t1 = ls[0]; ls[0] = gp_t1; ls[0] = gp_t0; }"},
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
		{"var j, k int\n\tgg[k] = append(gg[k], gg[j][0])", "gg[k] = gp::append(gg[k], gg[j][0]);"},
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
		{"fmt.Println(ga[0], f())", "{ auto gp_t0 = f(); gp::fmtshim::Println(ga[0], gp_t0); }"},
		{"la := [2]int{1, 2}\n\tfmt.Println(la[0], f())", "gp::fmtshim::Println(la[0], f());"},
		{"la := [2]int{1, 2}\n\ti := 0\n\tfmt.Println(la[i], f())", "{ auto gp_t0 = f(); gp::fmtshim::Println(la[i], gp_t0); }"},
		{"la := [2]bool{}\n\tfmt.Println(la[0], f())", "gp::fmtshim::Println(la[0], f());"},
		{"p := &ga\n\tfmt.Println(p[1], f())", "{ auto gp_t0 = f(); gp::fmtshim::Println((*p)[1], gp_t0); }"},
		{"fmt.Println(len(ga), f())", "gp::fmtshim::Println(int64_t(3), f());"},
		{"fmt.Println(len(mka()), f())", "{ auto gp_t0 = gp::len(mka()); gp::fmtshim::Println(gp_t0, f()); }"},
		{"fmt.Println(ga == [3]int{}, f())", "{ auto gp_t0 = ga == gp::array<int64_t, 3>{}; gp::fmtshim::Println(gp_t0, f()); }"},
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
		{"fmt.Println([2]int(gsl)[0], f())", "{ auto gp_t0 = f(); gp::fmtshim::Println(gp::to_array<2>(gsl)[0], gp_t0); }"},
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
		{"fmt.Println(gm[1], f())", "{ auto gp_t0 = f(); gp::fmtshim::Println(gp::get(gm, 1), gp_t0); }"},
		{"lm := map[int]int{}\n\tfmt.Println(lm[1], x)", "gp::fmtshim::Println(gp::get(lm, 1), x);"},
		{"lm := map[int]int{}\n\tfmt.Println(lm[1], f())", "{ auto gp_t0 = f(); gp::fmtshim::Println(gp::get(lm, 1), gp_t0); }"},
		{"lm := map[int]bool{}\n\tfmt.Println(lm[1], f())", "{ auto gp_t0 = gp::get(lm, 1); gp::fmtshim::Println(gp_t0, f()); }"},
		{"b := []byte(\"x\")\n\tlm := map[string]int{}\n\tfmt.Println(lm[string(b)], f())", "{ auto gp_t0 = gp::get(lm, gp::to_string(b)); gp::fmtshim::Println(gp_t0, f()); }"},
		{"lm := map[int]int{}\n\tfmt.Println(len(lm), f())", "{ auto gp_t0 = gp::len(lm); gp::fmtshim::Println(gp_t0, f()); }"},
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
		{"lm := map[int]int{}\n\tlm[1], lm[2] = lm[2], lm[1]", "{ auto gp_t0 = gp::get(lm, 2); auto gp_t1 = gp::get(lm, 1); gp::set(lm, 1, gp_t0); gp::set(lm, 2, gp_t1); }"},
		{"lm := map[int]int{}\n\tlm, lm[1] = nil, 2", "{ auto gp_t0 = lm; lm = nullptr; gp::set(gp_t0, 1, 2); }"},
		{"lm := map[int]int{1: 2, 3: f()}\n\t_ = lm", "gp::map<int64_t, int64_t> lm = gp::map_of<int64_t, int64_t>({{1, 2}, {3, f()}});"},
		{"lm := map[int]int{3: f(), 1: 2}\n\t_ = lm", "gp::map<int64_t, int64_t> lm = gp::map_of<int64_t, int64_t>({{1, 2}, {3, f()}});"},
		{"lm := map[int]int{f(): x, 2: f()}\n\t_ = lm", "gp::map<int64_t, int64_t> lm = gp::map_of<int64_t, int64_t>({{f(), x}, {2, f()}});"},
		{"lm := map[int]int{1: 2, x: f()}\n\t_ = lm", "gp::map<int64_t, int64_t> gp_t0 = gp::map_of<int64_t, int64_t>({{1, 2}}); auto gp_t1 = f(); gp::set(gp_t0, x, gp_t1); gp::map<int64_t, int64_t> lm = gp_t0;"},
		{"fmt.Println(len(map[int]int{1: x}), f())", "{ auto gp_t0 = gp::len(gp::map_of<int64_t, int64_t>({{1, x}})); gp::fmtshim::Println(gp_t0, f()); }"},
		{"for k, v := range gm {\n\t\t_, _ = k, v\n\t}", "for (auto gp_it0 = gp::range_map(gm); gp::range_next(gp_it0);) {"},
		{"delete(gm, f())", "{ auto gp_t0 = f(); gp::erase(gm, gp_t0); }"},
		{"lm := make(map[int]int, x)\n\t_ = lm", "gp::map<int64_t, int64_t> lm = gp::make_map<int64_t, int64_t>(x);"},
	}
	for _, tt := range tests {
		t.Run(tt.body, func(t *testing.T) {
			dir := t.TempDir()
			src := "package main\n\nimport \"fmt\"\n\nvar x = 1\n\nvar gsl = []int{1, 2}\n\nvar gg = [][]int{{1}}\n\nvar gm = map[int]int{}\n\nvar gms = map[int][]int{}\n\nfunc mk() []int {\n\treturn nil\n}\n\nfunc f() int {\n\tx++\n\tfmt.Print()\n\treturn x\n}\n\n" +
				"func g(a, b int) int {\n\treturn a + b\n}\n\nfunc h(p *int) *int {\n\treturn p\n}\n\n" +
				"var ga = [3]int{1, 2, 3}\n\nvar gg2 = [2][2]int{}\n\nfunc mka() *[3]int {\n\treturn nil\n}\n\nfunc main() {\n\t" + tt.body + "\n}\n"
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
