// phase: 08

// A parameter or a value receiver whose address only goes to borrowed pointer parameters (a
// pointer method, a function that borrows its pointer) stays a C++ stack local (ADR-114: escape.go
// counts them as locals): it is the function's own copy, so what the callee does through the
// pointer changes it and not the caller's value. One whose address escapes (returned, stored in a
// package variable, used by a goroutine or a deferred call, kept in a closure) still lives on the
// heap and outlives the call, which the reads after the calls check (and -asan would see a
// dangling pointer).
package main

import (
	"fmt"
	"sync"
)

type Counter struct {
	n    int
	hist [4]int
}

func (c *Counter) Inc()           { c.n++; c.hist[c.n%4]++ }
func (c *Counter) Self() *Counter { return c }
func bump(c *Counter)             { c.n += 10 }
func swap(a, b *Counter)          { a.n, b.n = b.n, a.n }

var global *Counter

func stays(c Counter) int {
	c.Inc()
	bump(&c)
	return c.n
}

func staysTwo(c, d Counter) int {
	swap(&c, &d)
	return c.n*100 + d.n
}

func (c Counter) Value() int {
	c.Inc()
	return c.n
}

func returned(c Counter) *Counter {
	c.Inc()
	return &c
}

func (c Counter) Pointer() *Counter { return c.Self() }

func stored(c Counter) {
	global = &c
}

func goroutine(c Counter, wg *sync.WaitGroup, out *int) {
	go func() {
		defer wg.Done()
		bump(&c)
		*out = c.n
	}()
}

func deferred(c Counter) (n int) {
	defer func() { n = c.n }()
	defer bump(&c)
	c.Inc()
	return 0
}

func closure(c Counter) func() int {
	return func() int {
		c.Inc()
		return c.n
	}
}

func main() {
	c := Counter{n: 1}
	fmt.Println(stays(c), c.n)
	fmt.Println(staysTwo(Counter{n: 1}, Counter{n: 2}))
	fmt.Println(c.Value(), c.n)
	p := returned(c)
	q := returned(Counter{n: 5})
	fmt.Println(p.n, q.n, c.n)
	r := c.Pointer()
	r.n = 99
	fmt.Println(r.n, c.n)
	stored(Counter{n: 7})
	stored2 := global
	stored(Counter{n: 8})
	fmt.Println(stored2.n, global.n)
	var wg sync.WaitGroup
	var out int
	wg.Add(1)
	goroutine(c, &wg, &out)
	wg.Wait()
	fmt.Println(out, c.n)
	fmt.Println(deferred(c), c.n)
	f := closure(c)
	fmt.Println(f(), f(), c.n)
}

// Output:
// 12 1
// 201
// 2 1
// 2 6 1
// 99 1
// 7 8
// 11 1
// 12 1
// 2 3 1
