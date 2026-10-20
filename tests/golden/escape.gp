// phase: 15

// Escape analysis (step 15.2, ADR-114): what stays on the C++ stack and what does not. In stack,
// c is an ordinary variable that the pointer methods and the borrowed parameters take by address
// (gp::borrow), p holds the address of c as a gp::stack_ptr, q is a gp::local_ptr that holds its
// object itself, and the &Counter{} that goes straight to a borrowed parameter is a temporary
// (gp::borrow_tmp). In heap everything escapes: c is boxed and d is gp::new_. The element variables
// of elements are const references into the slice, and the func literals of closures capture by
// reference.
package main

import "fmt"

type Counter struct{ n int }

func (c *Counter) Inc() { c.n++ }
func bump(c *Counter)   { c.n += 10 }

type Shape interface{ Area() int }

type Rect struct{ w, h int }

func (r *Rect) Area() int { return r.w * r.h }

var keep *Counter

func stack() int {
	var c Counter
	c.Inc()
	bump(&c)
	p := &c
	p.Inc()
	q := &Counter{n: 1}
	bump(q)
	bump(&Counter{n: 2})
	return c.n + q.n
}

func heap() *Counter {
	var c Counter
	keep = &c
	d := &Counter{}
	d.Inc()
	return d
}

func elements(shapes []Shape) int {
	sum := 0
	for _, s := range shapes {
		sum += s.Area()
	}
	first := shapes[0]
	return sum + first.Area()
}

func closures() int {
	n := 0
	add := func(k int) { n += k }
	add(1)
	add(2)
	func() { n *= 2 }()
	return n
}

func main() {
	fmt.Println(stack(), heap().n, elements([]Shape{&Rect{2, 3}}), closures())
}
