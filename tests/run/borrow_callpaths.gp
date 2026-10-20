// phase: 08

// The same callees reached through every call path: direct, method, method value, method expression, interface, closure, go and defer (ADR-086).
package main

import (
	"fmt"
	"sync"
)

type T struct {
	n    int
	name string
}

var g *T
var h struct{ p *T }
var arr [3]*T
var wg sync.WaitGroup
var sink int

func (t *T) use() int {
	g = nil
	h.p = nil
	arr[0] = nil
	return t.n + len(t.name)
}

func (t *T) use2(u *T) int {
	g = nil
	h.p = nil
	arr[0] = nil
	return t.n + len(t.name) + u.n
}

func use(t *T) int {
	g = nil
	h.p = nil
	arr[0] = nil
	return t.n + len(t.name)
}

type Shape interface{ Area() int }

func (t *T) Area() int { return t.use() }

func apply[X any](f func(*T) int, t *T) int { return f(t) }
func applyG[X any](x X, f func(X) int) int  { return f(x) }

type W struct {
	t *T
	f func(*T) int
}

func (w *W) call() int { return w.f(w.t) }

func reset() {
	g = &T{n: 1, name: "one"}
	h.p = g
	arr[0] = g
}

func main() {
	reset()
	fmt.Println((*T).use(g))
	reset()
	fmt.Println((*T).use2(h.p, arr[0]))
	reset()
	f := use
	fmt.Println(f(g))
	reset()
	m := g.use
	g = nil
	h.p = nil
	arr[0] = nil
	fmt.Println(m())
	reset()
	var s Shape = g
	g = nil
	h.p = nil
	arr[0] = nil
	fmt.Println(s.Area())
	reset()
	fmt.Println(apply[int](use, g))
	reset()
	fmt.Println(applyG(g, use))
	reset()
	w := &W{t: g, f: use}
	g = nil
	h.p = nil
	arr[0] = nil
	fmt.Println(w.call())
	reset()
	wg.Add(1)
	go func() { defer wg.Done(); sink = use(h.p) }()
	wg.Wait()
	fmt.Println(sink)
	reset()
	wg.Add(1)
	go func() { defer wg.Done(); sink = arr[0].use() }()
	wg.Wait()
	fmt.Println(sink)
	reset()
	func() {
		defer fmt.Println(use(g))
	}()
	reset()
	func() {
		defer func() { fmt.Println(sink) }()
		sink = h.p.use2(arr[0])
	}()
	reset()
	x := struct{ a, b int }{use(g), 7}
	fmt.Println(x)
	reset()
	fn := (*T).use2
	fmt.Println(fn(g, h.p))
	reset()
	fv := []func(*T) int{use, (*T).use}
	fmt.Println(fv[0](g))
	reset()
	fmt.Println(fv[1](g))
}

// Output:
// 4
// 5
// 4
// 4
// 4
// 4
// 4
// 4
// 4
// 4
// 4
// 5
// {4 7}
// 5
// 4
// 4
