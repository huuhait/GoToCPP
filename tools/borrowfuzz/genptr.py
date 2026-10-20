import random, sys
PRELUDE = r'''package main

import (
	"fmt"
	"sync"
)

type T struct {
	n    int
	name string
	next *T
	pad  [3]int
}

type I interface{ M(k int) int }

type H struct {
	p   *T
	arr [2]*T
	sl  []*T
	m   map[int]*T
	i   I
	f   func() *T
	ts  []T
}

var g0, g1 *T
var gh H
var gph *H
var gsl []*T
var gmp map[int]*T
var garr [3]*T
var gi I
var gfn func() *T
var gts []T
var gtarr [2]T
var gt T
var gkeep *T
var wg sync.WaitGroup
var logv []int

func fill() {
	mk := func(n int) *T { return &T{n: n, name: "obj", next: &T{n: n + 1000, name: "nx"}} }
	g0 = mk(1)
	g1 = mk(2)
	gh = H{p: mk(3), arr: [2]*T{mk(4), mk(5)}, sl: []*T{mk(6), mk(7)}, m: map[int]*T{1: mk(8)}, i: mk(9), ts: []T{{n: 10, name: "ts0"}, {n: 11, name: "ts1"}}}
	gh.f = func() *T { return gh.p }
	gph = &H{p: mk(12), arr: [2]*T{mk(13), mk(14)}, sl: []*T{mk(15)}, m: map[int]*T{1: mk(16)}, i: mk(17), ts: []T{{n: 18, name: "pts0"}}}
	gsl = []*T{mk(19), mk(20)}
	gmp = map[int]*T{1: mk(21), 2: mk(22)}
	garr = [3]*T{mk(23), mk(24), mk(25)}
	gi = mk(26)
	gfn = func() *T { return g0 }
	gts = []T{{n: 27, name: "gts0"}, {n: 28, name: "gts1"}}
	gtarr = [2]T{{n: 29, name: "gt0"}, {n: 30, name: "gt1"}}
	gt = T{n: 31, name: "gt"}
	gkeep = nil
}

func churn() {
	junk := make([]*T, 0, 16)
	for i := 0; i < 16; i++ {
		junk = append(junk, &T{n: -1, name: "junkjunkjunk", next: &T{n: -2}})
	}
	_ = junk
	_ = make([]T, 32)
}

func dropAll() {
	g0, g1 = nil, nil
	gh.p = nil
	gh.arr[0], gh.arr[1] = nil, nil
	gh.sl = nil
	gh.m = nil
	gh.i = nil
	gh.f = nil
	gh.ts = nil
	if gph != nil {
		gph.p = nil
		gph.arr[0] = nil
		gph.sl = nil
		gph.ts = nil
	}
	gph = nil
	gsl = nil
	gmp = nil
	garr = [3]*T{}
	gi = nil
	gfn = nil
	gts = nil
	gtarr = [2]T{}
	gt = T{}
	churn()
}

func use(t *T) int {
	s := t.n + len(t.name)
	if t.next != nil {
		s += t.next.n
	}
	return s
}

func (t *T) M(k int) int { dropAll(); churn(); t.n++; return use(t) + k }

func (t *T) m0(k int) int { dropAll(); churn(); t.n++; logv = append(logv, t.n); return use(t) + k }
func (t *T) m1(k int, xs ...int) int {
	dropAll()
	churn()
	return use(t) + k + len(xs)
}
func (t *T) m2(k int, f func()) int { f(); dropAll(); churn(); return use(t) + k }
func (t *T) m3(k int, o *T) int     { dropAll(); churn(); return use(t) + o.n + k }
func (t *T) m4(k int, h *H) int {
	h.p = nil
	h.arr[0] = nil
	dropAll()
	churn()
	return use(t) + k
}
func (t *T) m5(k int, i I) int {
	r := i.M(k)
	churn()
	return use(t) + r
}
func (t *T) mg(k int) int { defer wg.Done(); dropAll(); churn(); return use(t) + k }
func (t *T) pure(k int) int {
	s := 0
	for i := range t.pad {
		t.pad[i] += k
		s += t.pad[i]
	}
	return s + use(t)
}
func (t *T) kept(k int) int { gkeep = t; dropAll(); churn(); return use(t) + k }
func (t T) val(k int) int   { dropAll(); churn(); return use(&t) + k }
func (t *T) viaPure(k int) int {
	if t.next == nil {
		return t.pure(k)
	}
	return t.pure(k) + t.next.pure(k)
}

func f0(t *T, k int) int { dropAll(); churn(); t.n++; logv = append(logv, t.n); return use(t) + k }
func f1(t *T, k int, xs ...int) int {
	dropAll()
	churn()
	return use(t) + k + len(xs)
}
func f2(t *T, k int, f func()) int { f(); dropAll(); churn(); return use(t) + k }
func f3(t, o *T, k int) int         { dropAll(); churn(); return use(t) + o.n + k }
func f4(t *T, h *H, k int) int {
	h.p = nil
	dropAll()
	churn()
	return use(t) + k
}
func f5(t *T, k int, i I) int {
	r := i.M(k)
	churn()
	return use(t) + r
}
func fg(t *T, k int) int { defer wg.Done(); dropAll(); churn(); return use(t) + k }
func fpure(t *T, k int) int { return use(t) + t.pad[0] + k }
func fkept(t *T, k int) int { gkeep = t; dropAll(); churn(); return use(t) + k }

func two(t *T) (*T, int) { return t, 7 }

func flush() {
	fmt.Println(logv)
	logv = nil
}

'''
RECVS = ["g0", "g1", "gh.p", "gh.arr[0]", "gh.arr[1]", "gh.sl[0]", "gh.sl[1]", "gh.m[1]", "gph.p", "gph.arr[1]", "gph.sl[0]", "gph.m[1]",
         "gsl[0]", "gsl[1]", "gmp[1]", "gmp[2]", "garr[0]", "garr[2]", "gh.f()", "gfn()", "gh.p.next", "g0.next", "gph.p.next", "gl", "gb", "gi.(*T)", "gh.i.(*T)", "gkeepv"]
SREC = ["gts[0]", "gts[1]", "gtarr[0]", "gtarr[1]", "gt", "gh.ts[0]", "gph.ts[0]", "(*gph).ts[0]"]
METHS = {  # name: (extra args template, kind)
 "m0": "@K@", "m1": "@K@, 1, 2", "m1b": "@K@", "m2": "@K@, dropAll", "m2b": "@K@, func() { dropAll(); g0 = nil }", "m3": "@K@, @R2@", "m4": "@K@, &gh", "m4b": "@K@, gph", "m5": "@K@, gi", "m5b": "@K@, @R2@", "pure": "@K@", "kept": "@K@", "val": "@K@", "viaPure": "@K@"}
FUNCS = {"f0": "@K@", "f1": "@K@, 1", "f1b": "@K@", "f2": "@K@, dropAll", "f3": "@R2@, @K@", "f4": "&gh, @K@", "f4b": "gph, @K@", "f5": "@K@, gi", "fpure": "@K@", "fkept": "@K@"}

def gen(seed, n=30):
    r = random.Random(seed)
    out = [PRELUDE]
    out.append("func main() {\n")
    out.append("\tvar gl, gb, gkeepv *T\n\t_ = gl\n\t_ = gb\n\t_ = gkeepv\n")
    lines = []
    for i in range(n):
        R = r.choice(RECVS + RECVS + SREC)
        R2 = r.choice(RECVS)
        k = r.randint(0, 9)
        isS = R in SREC
        pre = ["fill()"]
        # locals that hold an owner: gl local copy, gb captured by a closure
        pre.append("gl = g0; gb = gh.p; gkeepv = gsl[0]")
        pre.append("gbf := func() { gb = nil }; _ = gbf")
        form = r.choice(["call", "call", "call", "mv", "mexpr", "fvar", "defer", "go", "iface", "tuple", "val", "fn", "fnmv"])
        body = ""
        if isS:
            m = r.choice(["m0", "m1b", "m3", "pure", "kept", "m2", "m5b"])
            form = r.choice(["call", "call", "mv", "mexpr", "defer", "go", "fnmv"]) if m != "kept" else "call"
        else:
            m = r.choice(list(METHS))
        if form == "fn":
            fn = r.choice(list(FUNCS)); a = FUNCS[fn].replace('@K@',str(k)).replace('@R2@',R2)
            if fn == "f1b": a = "%d" % k
            if fn in ("f1","f1b","f2","f3","f4","f4b","f5") and fn in ("f3","f4","f4b"): pass
            # function: first param is t
            argl = FUNCS[fn].replace('@K@',str(k)).replace('@R2@',R2)
            if fn == "f1b": argl = str(k)
            if fn in ("f3",):
                call = "f3(%s, %s, %d)" % (R, R2, k)
            elif fn in ("f4", "f4b"):
                call = "%s(%s, %s, %d)" % ("f4", R, "&gh" if fn=="f4" else "gph", k)
            elif fn == "f1b":
                call = "f1(%s, %d)" % (R, k)
            else:
                rest = argl
                call = "%s(%s, %s)" % (fn, R, rest)
            body = "fmt.Println(%s)" % call
        elif form == "fnmv":
            # func value of a function and calling it
            fn = r.choice(["f0", "fpure", "fkept"])
            body = "ff := %s; fmt.Println(ff(%s, %d))" % (fn, R, k)
            if isS:
                body = "ff := (*T).m0; fmt.Println(ff(&%s, %d))" % (R, k)
        else:
            a = METHS[m].replace('@K@',str(k)).replace('@R2@',R2)
            if m == "m1b": a = str(k)
            mname = {"m1b": "m1", "m2b": "m2", "m4b": "m4", "m5b": "m5"}.get(m, m)
            if m == "m3" and isS: a = "%d, g1" % k
            if m == "m5b": a = "%d, %s" % (k, "gi") if isS else "%d, %s" % (k, R2)
            if m == "m4": a = "%d, &gh" % k
            if m == "m4b": a = "%d, gph" % k
            if m == "m2b": a = "%d, func() { dropAll(); g0 = nil }" % k
            if m == "m3" and not isS: a = "%d, %s" % (k, R2)
            if form == "call":
                body = "fmt.Println(%s.%s(%s))" % (R, mname, a)
            elif form == "mv":
                body = "mv := %s.%s; fmt.Println(mv(%s))" % (R, mname, a)
            elif form == "mexpr":
                recv = "&" + R if isS else R
                body = "fmt.Println((*T).%s(%s, %s))" % (mname, recv, a)
                if mname == "val": body = "fmt.Println(T.val(*%s, %s))" % (recv.lstrip("&") if isS else R, a) if False else "fmt.Println(%s.val(%s))" % (R, a)
            elif form == "fvar":
                recv = "&" + R if isS else R
                body = "fv := (*T).%s; fmt.Println(fv(%s, %s))" % (mname, recv, a)
                if mname == "val": body = "fmt.Println(%s.val(%s))" % (R, a)
            elif form == "defer":
                body = "func() { defer %s.%s(%s) }()" % (R, mname, a)
                if m in ("val",): body = "func() { defer %s.%s(%s) }()" % (R, mname, a)
            elif form == "go":
                body = "wg.Add(1); go %s.mg(%d); wg.Wait()" % (R, k)
            elif form == "iface":
                if isS:
                    body = "fmt.Println(%s.m0(%d))" % (R, k)
                else:
                    body = "var x I = %s; fmt.Println(x.M(%d))" % (R, k)
            elif form == "tuple":
                if isS:
                    body = "fmt.Println(%s.m0(%d))" % (R, k)
                else:
                    body = "fmt.Println(f3Tuple(two(%s)))" % R
            elif form == "val":
                body = "fmt.Println(%s.%s(%s))" % (R, mname, a)
        lines.append("\t{\n\t\t" + "\n\t\t".join(pre) + "\n\t\t" + body + "\n\t\tflush()\n\t}\n")
    out.extend(lines)
    out.append("}\n\nfunc f3Tuple(t *T, k int) int { return f0(t, k) }\n")
    return "".join(out)

if __name__ == "__main__":
    seed = int(sys.argv[1]); n = int(sys.argv[2]) if len(sys.argv) > 2 else 30
    sys.stdout.write(gen(seed, n))
