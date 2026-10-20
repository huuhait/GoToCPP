// phase: 08

// Functions and methods with a borrowed pointer parameter (ADR-086) used as values: method values
// bound to a pointer that is reassigned afterwards, method expressions, func values of plain
// functions, a deferred call, a goroutine started with a method on a pointer that the caller then
// drops, a function passed as a callback, and a call with the results of another call.
package main

import (
	"fmt"
	"sync"
)

type T struct {
	n    int
	name string
}

func (t *T) Inc() int         { t.n++; return t.n }
func (t *T) Add(o *T) int     { t.n += o.n; return t.n }
func (t *T) Name() string     { return t.name }
func get(t *T) int            { return t.n }
func add2(a, b *T) int        { return a.n + b.n }
func apply(f func(*T) int, t *T) int { return f(t) }
func pair() (*T, *T)          { return &T{n: 1}, &T{n: 2} }
func (t *T) Both(a, b *T) int { return t.n + a.n + b.n }

var done = make(chan int, 4)

func (t *T) run(wg *sync.WaitGroup) {
	defer wg.Done()
	t.n += 100
	done <- t.n
}

func work(t *T, wg *sync.WaitGroup) {
	defer wg.Done()
	t.n += 200
	done <- t.n
}

func main() {
	p := &T{n: 10, name: "p"}
	inc := p.Inc // bound to the object p points to now
	p = &T{n: 50, name: "q"}
	fmt.Println(inc(), inc(), p.n)
	fmt.Println(p.Inc(), inc())

	q := &T{n: 1, name: "q"}
	add := q.Add
	other := &T{n: 5}
	fmt.Println(add(other), add(other))
	other = nil
	fmt.Println(add(&T{n: 1}))

	f := get
	fmt.Println(f(p), apply(get, q), apply(f, &T{n: 33}))
	g := add2
	fmt.Println(g(p, q))

	me := (*T).Inc
	ma := (*T).Add
	fmt.Println(me(p), ma(p, q), (*T).Name(q))

	// A deferred call evaluates its receiver and arguments when it is deferred.
	func() {
		r := &T{n: 1}
		s := &T{n: 2}
		defer fmt.Println("deferred", r.Add(s), get(r))
		defer r.Inc()
		defer func(t *T) { fmt.Println("closure", t.n) }(r)
		r = nil
		s.n = 99
	}()

	// A goroutine started with a method on a pointer that the caller drops.
	var wg sync.WaitGroup
	h, h2 := &T{n: 1}, &T{n: 2}
	wg.Add(2)
	go h.run(&wg)
	go work(h2, &wg)
	h, h2 = nil, nil
	wg.Wait()
	a, b := <-done, <-done
	fmt.Println(min(a, b) > 100, a+b > 0)

	// The results of a call as the arguments.
	fmt.Println(add2(pair()), (&T{n: 4}).Both(pair()))
	var t0 T
	fmt.Println(t0.Both(pair()), t0.n)

	// A callback table.
	table := map[string]func(*T) int{"get": get, "inc": (*T).Inc}
	c := &T{n: 7}
	fmt.Println(table["get"](c), table["inc"](c), table["get"](c))
}

// Output:
// 11 12 50
// 51 13
// 6 11
// 12
// 51 12 33
// 63
// 52 64 q
// closure 3
// deferred 3 3
// true true
// 3 7
// 3 0
// 7 8 8
