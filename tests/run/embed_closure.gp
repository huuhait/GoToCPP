// phase: 05

// Closures and method values capture the whole object: a promoted field or method used in a
// func literal reads and writes the same memory as the outer variable, also after the function
// that made it has returned.
package main

import "fmt"

type Counter struct{ n int }

func (c *Counter) Inc() int { c.n++; return c.n }
func (c Counter) Get() int  { return c.n }

type Svc struct {
	Counter
	name string
}

type PSvc struct {
	*Counter
	name string
}

func counterFuncs() (func() int, func() int) {
	s := Svc{name: "local"}
	return func() int { return s.Inc() }, func() int { return s.n }
}

func ptrFuncs(c *Counter) (func(), func() int) {
	p := PSvc{c, "p"}
	return func() { p.n += 10 }, p.Get // Get copies *c now
}

func main() {
	inc, get := counterFuncs()
	inc()
	inc()
	fmt.Println(get(), inc(), get())

	c := &Counter{}
	add, snap := ptrFuncs(c)
	add()
	c.Inc()
	fmt.Println(snap(), c.n)

	s := Svc{}
	f := func() { s.Counter.n = 100; s.Inc() }
	f()
	g := s.Get
	s.Inc()
	fmt.Println(s.n, g())
	fs := make([]func() int, 3)
	for i := range fs {
		fs[i] = s.Inc
	}
	fmt.Println(fs[0](), fs[1](), fs[2](), s.n)
}

// Output:
// 2 3 3
// 0 11
// 102 101
// 103 104 105 105
