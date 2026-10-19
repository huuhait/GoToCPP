// phase: 08

// Method values and method expressions of generic types: bound receivers (value copies and
// pointers), expressions that take the receiver first, the same through an interface, stored in
// maps and slices, called later after the receiver changed, deferred and started as goroutines.
package main

import (
	"fmt"
	"slices"
	"sync"
)

type Counter[T ~int | ~int8] struct {
	n    T
	name string
}

func (c Counter[T]) Get() T                { return c.n }
func (c *Counter[T]) Inc() T               { c.n++; return c.n }
func (c *Counter[T]) Add(d T) *Counter[T]  { c.n += d; return c }
func (c Counter[T]) Label(p string) string { return fmt.Sprint(p, c.name, c.n) }

type Getter[T any] interface{ Get() T }

func Collect[T any](gs ...Getter[T]) (out []T) {
	for _, g := range gs {
		out = append(out, g.Get())
	}
	return
}

func main() {
	c := Counter[int8]{126, "c"}
	get := c.Get // copies c now
	inc := c.Inc // binds &c
	label := c.Label
	fmt.Println(get(), inc(), inc(), inc(), c.n, get(), label(">"))
	fe := Counter[int8].Get
	fi := (*Counter[int8]).Inc
	fl := Counter[int8].Label
	fg := (*Counter[int8]).Get // pointer method set includes value methods
	fmt.Println(fe(c), fi(&c), fi(&c), fl(c, "#"), fg(&c))
	fmt.Printf("%T | %T | %T\n", fe, fi, fl)
	ops := map[string]func(*Counter[int]) int{
		"inc": (*Counter[int]).Inc,
		"get": func(c *Counter[int]) int { return c.Get() },
	}
	k := &Counter[int]{n: 40}
	keys := make([]string, 0)
	for name := range ops {
		keys = append(keys, name)
	}
	slices.Sort(keys)
	for _, name := range keys {
		fmt.Println(name, ops[name](k), ops[name](k))
	}
	var g Getter[int8] = c
	gm := g.Get
	c.Add(100).Add(100)
	fmt.Println(gm(), g.Get(), c.Get(), Collect[int8](c, &c, Counter[int8]{n: 5}))
	chain := k.Add(2).Add(3)
	fmt.Println(chain == k, k.n)

	var wg sync.WaitGroup
	d := &Counter[int]{n: 0}
	func() {
		defer d.Add(5) // evaluated receiver now, run at return
		defer func() { fmt.Println("deferred sees", d.n) }()
		d.Inc()
	}()
	fmt.Println(d.n)
	res := make([]int, 4)
	for i := range res {
		wg.Add(1)
		cc := Counter[int]{n: i * 10}
		go func() {
			defer wg.Done()
			f := cc.Inc
			f()
			res[i] = f() + cc.Get()
		}()
	}
	wg.Wait()
	fmt.Println(res)
	var nilc *Counter[int]
	f := nilc.Add
	fmt.Println(f != nil)
	func() {
		defer func() { fmt.Println("recovered:", recover() != nil) }()
		f(1)
	}()
}

// Output:
// 126 127 -128 -127 -127 126 >c126
// -127 -126 -125 #c-125 -125
// func(main.Counter[int8]) int8 | func(*main.Counter[int8]) int8 | func(main.Counter[int8], string) string
// get 40 40
// inc 41 42
// -125 -125 75 [75 75 5]
// true 47
// deferred sees 1
// 6
// [4 24 44 64]
// true
// recovered: true
