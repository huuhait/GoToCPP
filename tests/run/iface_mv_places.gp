// phase: 05

// Method values of interface methods as arguments, results, struct fields, map values, slice
// elements and closure captures (step 5.9).
package main

import "fmt"

type Counter interface {
	Inc() int
	Add(n int) int
}

type Cn struct{ n int }

func (c *Cn) Inc() int      { c.n++; return c.n }
func (c *Cn) Add(n int) int { c.n += n; return c.n }

type Box struct {
	inc func() int
	add func(int) int
	c   Counter
}

func call(f func() int) int            { return f() }
func call2(f func(int) int, x int) int { return f(x) }

func pick(c Counter, second bool) func() int {
	if second {
		return func() int { return c.Inc() * 100 }
	}
	return c.Inc
}

func main() {
	c := &Cn{}
	var i Counter = c
	fmt.Println(call(i.Inc), call(i.Inc), call2(i.Add, 10), c.n)

	b := Box{inc: i.Inc, add: i.Add, c: i}
	fmt.Println(b.inc(), b.add(5), b.c.Inc(), c.n)

	m := map[string]func() int{"inc": i.Inc, "twice": func() int { i.Inc(); return i.Inc() }}
	fmt.Println(m["inc"](), m["twice"](), c.n)

	fs := []func(int) int{i.Add, i.Add}
	fmt.Println(fs[0](1), fs[1](1), c.n)

	fmt.Println(pick(i, false)(), pick(i, true)(), c.n)

	cl := func() func() int {
		inc := i.Inc
		return func() int { return inc() + inc() }
	}()
	fmt.Println(cl(), c.n)

}

// Output:
// 1 2 12 12
// 13 18 19 19
// 20 22 22
// 23 24 24
// 25 2600 26
// 55 28
