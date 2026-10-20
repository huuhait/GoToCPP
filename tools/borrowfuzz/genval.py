import random, sys
PRELUDE = r'''package main

import (
	"fmt"
	"strings"
	"sync"
)

type T struct {
	n    int
	name string
}

type V struct {
	s  string
	xs []int
	a  any
	f  func() int
	t  *T
	m  map[string]int
}

var gstr string
var gslice []int
var gany any
var gfun func() int
var gv V
var gpv *V
var gvs []V
var gvm map[string]string
var gstrs []string
var gt *T
var gmi map[string]int
var gar [2]string
var wg sync.WaitGroup
var logv []string

func rep(s string) string { return strings.Repeat(s, 3) }

func resetV() {
	gstr = rep("ab")
	gslice = []int{1, 2, 3}
	gany = 42
	gfun = func() int { return 7 }
	gv = V{s: rep("vs"), xs: []int{4, 5}, a: "anystr", f: func() int { return 8 }, t: &T{n: 9, name: "tt"}, m: map[string]int{"a": 1}}
	gpv = &V{s: rep("ps"), xs: []int{6, 7, 8}, a: 99, f: func() int { return 9 }, t: &T{n: 10, name: "ptt"}, m: map[string]int{"b": 2}}
	gvs = []V{{s: rep("e0"), xs: []int{1}, a: 1.5}, {s: rep("e1"), xs: []int{2, 3}, a: "x"}}
	gvm = map[string]string{"k": rep("mv")}
	gstrs = []string{rep("s0"), rep("s1")}
	gt = &T{n: 11, name: "gt"}
	gmi = map[string]int{"z": 1}
	gar = [2]string{rep("ar0"), rep("ar1")}
}

func churnV() {
	for i := 0; i < 20; i++ {
		_ = rep("churnchurn")
		_ = make([]int, 40)
		_ = []any{1, "x", 2.5}
	}
}

func dropV() {
	gstr = "changedG"
	gslice = []int{100}
	gany = "changedAny"
	gfun = func() int { return -1 }
	gv.s = "changedVS"
	gv.xs = nil
	gv.a = nil
	gv.f = nil
	gv.t = nil
	gv.m = nil
	if gpv != nil {
		gpv.s = "changedPS"
		gpv.xs = []int{9}
		gpv.a = nil
		gpv.f = nil
		gpv.t = nil
	}
	gpv = nil
	gvs = nil
	gvm["k"] = "changedMV"
	gstrs = nil
	gt = nil
	gmi = nil
	gar[0], gar[1] = "ar0x", "ar1x"
	churnV()
}

func show(s string, xs []int, a any, f func() int, t *T) string {
	r := s + fmt.Sprint(xs, a)
	if f != nil {
		r += fmt.Sprint(f())
	}
	if t != nil {
		r += fmt.Sprint(t.n, t.name)
	}
	return r
}

func (o *T) s0(s string) string { dropV(); return s + "|" + fmt.Sprint(o.n) }
func (o *T) s1(xs []int) string { dropV(); churnV(); return fmt.Sprint(xs, o.n) }
func (o *T) s2(a any) string    { dropV(); return fmt.Sprint(a, o.n) }
func (o *T) s3(f func() int) string {
	dropV()
	if f == nil {
		return "nilf"
	}
	return fmt.Sprint(f(), o.n)
}
func (o *T) s4(t *T) string { dropV(); return fmt.Sprint(t.n, t.name, o.n) }
func (o *T) s5(s string, xs []int, a any, f func() int, t *T) string {
	dropV()
	return show(s, xs, a, f, t) + fmt.Sprint(o.n)
}
func (o *T) s6(s string) string { gstr = "m6"; gv.s = "m6"; gstrs = nil; return s + "!" }
func (o *T) s7(s string) string { o.name = "mutated" + o.name; return s + o.name }
func (o *T) pureS(s string, xs []int) string { return s + fmt.Sprint(len(xs), o.n) }
func (o *T) pureS2(s string, xs []int) int {
	t := 0
	for _, x := range xs {
		t += x
	}
	return t + len(s) + o.n
}
func (o *T) keepS(s string) string { logv = append(logv, s); dropV(); return s + "|" + fmt.Sprint(len(logv)) }

func (v *V) v0(s string) string  { v.s = "v0changed"; return s + "!" }
func (v *V) v1(xs []int) string  { v.xs = nil; return fmt.Sprint(xs) }
func (v *V) v2(a any) string     { v.a = "v2"; return fmt.Sprint(a) }
func (v *V) v3(f func() int) int { v.f = nil; return f() }
func (v *V) v4(t *T) int         { v.t = nil; return t.n }
func (v *V) v5(s string, o *V) string {
	o.s = "v5"
	v.s = "v5b"
	return s
}

func f0(s string) string { dropV(); return s + "!" }
func f1(xs []int) string { dropV(); return fmt.Sprint(xs) }
func f2(a any) string    { dropV(); return fmt.Sprint(a) }
func f3(f func() int) int {
	dropV()
	if f == nil {
		return -99
	}
	return f()
}
func f4(t *T) string { dropV(); return fmt.Sprint(t.n, t.name) }
func f5(s string, xs []int, a any, f func() int, t *T) string {
	dropV()
	return show(s, xs, a, f, t)
}
func f6(a, b string) string { gstr = "f6"; gstrs = nil; return a + b }
func f7(xs, ys []int) string {
	ys[0] = 77
	return fmt.Sprint(xs, ys)
}
func f8(v *V, s string) string { v.s = "f8"; return s + "!" }
func f9(s string) string { gar[0] = "f9x"; return s + "!" }

func flushV() {
	fmt.Println(logv)
	logv = nil
}

'''
# argument expressions per kind
STR = ["gstr", "gv.s", "gpv.s", "gvs[0].s", "gvs[1].s", "gvm[\"k\"]", "gstrs[1]", "gar[0]", "gt.name", "gv.t.name", "(gstr)", "gstr + \"x\"", "gstr[1:]", "string(gstr)", "gpv.t.name"]
SLICE = ["gslice", "gv.xs", "gpv.xs", "gvs[1].xs", "gslice[1:]", "gv.xs[:1]"]
ANY = ["gany", "gv.a", "gpv.a", "gvs[0].a", "any(gstr)", "any(gt)"]
FUN = ["gfun", "gv.f", "gpv.f"]
PTR = ["gt", "gv.t", "gpv.t"]
RECV = ["gt", "gv.t", "gpv.t"]
VREC = ["&gv", "gpv", "&gvs[0]", "&gvs[1]"]
VRECV = ["gv", "gpv", "gvs[0]", "gvs[1]"]

def gen(seed, n=30):
    r = random.Random(seed)
    out = [PRELUDE, "func main() {\n"]
    for i in range(n):
        form = r.choice(["call", "call", "mv", "mexpr", "fvar", "defer", "fn", "fnv", "go"])
        kind = r.choice(["s0", "s1", "s2", "s3", "s4", "s5", "s6", "s7", "pureS", "pureS2", "keepS", "v0", "v1", "v2", "v3", "v4", "v5", "f0", "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9"])
        R = r.choice(RECV); vi = r.randrange(4); VR = VRECV[vi]; VRP = VREC[vi]
        a = {"s0": [STR], "s1": [SLICE], "s2": [ANY], "s3": [FUN], "s4": [PTR], "s5": [STR, SLICE, ANY, FUN, PTR], "s6": [STR], "s7": [STR], "pureS": [STR, SLICE], "pureS2": [STR, SLICE], "keepS": [STR],
             "v0": [STR], "v1": [SLICE], "v2": [ANY], "v3": [FUN], "v4": [PTR], "v5": [STR, VREC],
             "f0": [STR], "f1": [SLICE], "f2": [ANY], "f3": [FUN], "f4": [PTR], "f5": [STR, SLICE, ANY, FUN, PTR], "f6": [STR, STR], "f7": [SLICE, SLICE], "f8": [VREC, STR], "f9": [STR]}[kind]
        args = ", ".join(r.choice(x) for x in a)
        isv = kind.startswith("v"); isf = kind.startswith("f")
        T = "V" if isv else "T"
        recv = VR if isv else R
        if isf:
            if form in ("call", "mv", "mexpr", "fvar"): form = "fn"
            if form == "go": form = "fn"
            if form == "defer": body = "func() { defer %s(%s) }()" % (kind, args) if kind != "f0" else "func() { defer fmt.Println(%s(%s)) }()" % (kind, args)
            elif form == "fnv": body = "ff := %s; fmt.Println(ff(%s))" % (kind, args)
            else: body = "fmt.Println(%s(%s))" % (kind, args)
        else:
            if form == "fn": form = "call"
            if form == "fnv": form = "fvar"
            if form == "call": body = "fmt.Println(%s.%s(%s))" % (recv, kind, args)
            elif form == "mv": body = "mv := %s.%s; fmt.Println(mv(%s))" % (recv, kind, args)
            elif form == "mexpr": body = "fmt.Println((*%s).%s(%s, %s))" % (T, kind, VRP if isv else recv, args)
            elif form == "fvar": body = "fv := (*%s).%s; fmt.Println(fv(%s, %s))" % (T, kind, VRP if isv else recv, args)
            elif form == "defer": body = "func() { defer %s.%s(%s) }()" % (recv, kind, args)
            else: body = "fmt.Println(%s.%s(%s))" % (recv, kind, args)
        out.append("\t{\n\t\tresetV()\n\t\t%s\n\t\tflushV()\n\t}\n" % body)
    out.append("}\n")
    return "".join(out)

if __name__ == "__main__":
    sys.stdout.write(gen(int(sys.argv[1]), int(sys.argv[2]) if len(sys.argv) > 2 else 30))
