// phase: 08

// Parameters that the callee never assigns are const references (ADR-086): strings, slices, maps,
// channels, funcs, interfaces and pointers that are not borrowed. The caller passes a local that
// nothing can change as it is, and copies anything else (a package variable, a field, an element,
// a boxed variable) before the call, so a callee that changes the source while it runs still sees
// the value it was called with, as in Go. The callee here changes the source and goes on using its
// parameter.
package main

import (
	"fmt"
	"sync"
)

type Rec struct {
	name  string
	items []int
	next  *Rec
	cb    func() int
}

type Iface interface{ M() int }

type Num int

func (n Num) M() int { return int(n) }

type IDs []int

func (ids IDs) Sum() (s int) {
	for _, v := range ids {
		s += v
	}
	return
}

var (
	gs    = "global"
	gx    []int
	gm    map[string]int
	gf    func() int
	gi    Iface
	grec  *Rec
	gname string
	gids  IDs
)

func useStr(s string) string { gs = "changed"; return s }
func useSlice(xs []int) int  { gx = nil; return len(xs) + xs[0] }
func useMap(m map[string]int) int {
	gm = nil
	return m["a"]
}
func useFunc(f func() int) int { gf = nil; return f() }
func useIface(i Iface) int    { gi = nil; return i.M() }
func useIDs(ids IDs) int      { gids = nil; return ids.Sum() }

// A pointer that is kept (so not borrowed) and a package variable that held it.
var kept *Rec

func keep(r *Rec) string {
	grec = nil
	kept = r
	return r.name
}

// The source is a field of the object that the callee changes.
func rename(r *Rec, s string) string { r.name = "renamed"; return s }
func reset(r *Rec, xs []int) int     { r.items = nil; return len(xs) }
func setcb(r *Rec, f func() int) int { r.cb = nil; return f() }
func unlink(r *Rec, n *Rec) string   { r.next = nil; kept = n; return n.name }

// A callee that returns or captures its parameter.
func echo(s string) string { return s }
func mk(s string) func() string {
	return func() string { return s }
}
func tail(xs []int) []int { return xs[1:] }

// A callee that assigns its parameter keeps a copy of its own.
func shout(s string) string {
	s += "!"
	return s
}

func recur(s string, n int) int {
	if n == 0 {
		return len(s)
	}
	gs = gs + "."
	return recur(s, n-1) + 1
}

func variadic(xs ...int) int { gx = nil; return len(xs) + xs[0] }

func nils(s string, xs []int, m map[string]int, f func(), i Iface, r *Rec, c chan int) string {
	return fmt.Sprint(len(s), xs == nil, m == nil, f == nil, i == nil, r == nil, c == nil)
}

func (r *Rec) cmp(s string) bool  { r.name = "other"; return s == "orig" }
func (r *Rec) set(xs []int) int  { r.items = nil; return len(xs) }
func (r *Rec) self(o *Rec) *Rec  { kept = r; return o }
func (r Rec) val(s string) string { gs = "v"; return r.name + s }

func show(tag string, s string, xs []int) {
	fmt.Println(tag, s, xs)
}

func main() {
	fmt.Println(useStr(gs), gs)
	gx = []int{4, 5}
	fmt.Println(useSlice(gx), gx == nil)
	gm = map[string]int{"a": 7}
	fmt.Println(useMap(gm), gm == nil)
	gf = func() int { return 8 }
	fmt.Println(useFunc(gf), gf == nil)
	gi = Num(9)
	fmt.Println(useIface(gi), gi == nil)
	gids = IDs{1, 2, 3}
	fmt.Println(useIDs(gids), gids == nil)

	grec = &Rec{name: "kept"}
	fmt.Println(keep(grec), grec == nil, kept.name)

	r := &Rec{name: "orig", items: []int{1, 2, 3}, cb: func() int { return 10 }, next: &Rec{name: "next"}}
	fmt.Println(rename(r, r.name), r.name)
	fmt.Println(reset(r, r.items), r.items == nil)
	fmt.Println(setcb(r, r.cb), r.cb == nil)
	fmt.Println(unlink(r, r.next), r.next == nil)

	r.name = "orig"
	fmt.Println(r.cmp(r.name), r.name)
	r.items = []int{1, 2}
	fmt.Println(r.set(r.items), r.items == nil)
	gs = "g"
	fmt.Println(r.val(gs), gs)

	// The local variable that the callee cannot change is passed as it is.
	local := "local"
	xs := []int{1, 2, 3}
	fmt.Println(echo(local), tail(xs), len(xs), shout(local), local)

	// A closure that captures the parameter keeps its own copy.
	gs = "before"
	f := mk(gs)
	gs = "after"
	fmt.Println(f(), gs)

	gs = "x"
	fmt.Println(recur(gs, 3), gs)
	gx = []int{6, 7, 8}
	fmt.Println(variadic(gx...), gx == nil)
	gx = []int{6, 7, 8}
	fmt.Println(variadic(1, 2, 3))
	fmt.Println(nils("", nil, nil, nil, nil, nil, nil))
	fmt.Println(nils("a", []int{}, map[string]int{}, func() {}, Num(1), &Rec{}, make(chan int)))

	// Func values of such functions, and method values, expressions.
	h := useStr
	e := echo
	gs = "val"
	fmt.Println(h(gs), e("e"), fmt.Sprintf("%p", useStr) == fmt.Sprintf("%p", h))
	m := r.cmp
	fmt.Println(m("orig"), (*Rec).cmp(r, "zz"))
	fs := []func(string) string{echo, shout, useStr}
	var res []string
	for _, fn := range fs {
		gs = "loop"
		res = append(res, fn(gs))
	}
	fmt.Println(res)

	// A deferred call and a goroutine evaluate their arguments when they are started.
	func() {
		gs = "deferred"
		gx = []int{9}
		defer show("defer", gs, gx)
		gs = "late"
		gx[0] = 10
		gx = nil
	}()
	var wg sync.WaitGroup
	out := make([]string, 2)
	gs, gx = "go", []int{1, 2}
	wg.Add(2)
	go func(s string) { defer wg.Done(); out[0] = s }(gs)
	go func(s string, xs []int) { defer wg.Done(); out[1] = fmt.Sprint(s, xs) }(gs, gx)
	gs, gx = "changed", nil
	wg.Wait()
	fmt.Println(out)
}

// Output:
// global changed
// 6 true
// 7 true
// 8 true
// 9 true
// 6 true
// kept true kept
// orig renamed
// 3 true
// 10 true
// next true
// true other
// 2 true
// otherg v
// local [2 3] 3 local! local
// before after
// 4 x...
// 9 true
// 4
// 0 true true true true true true
// 1 false false false false false false
// val e true
// true false
// [loop loop! loop]
// defer deferred [10]
// [go go[1 2]]
