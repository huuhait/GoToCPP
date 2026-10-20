// bench: escape
// category: dispatch
// size: 50000000
// smoke: 50000
// twin: no
// about: escape analysis (ADR-114): address-taken locals and &T{} passed to borrowed parameters, local closures, and interface elements read from a slice
package main

import "fmt"

type Counter struct{ n int }

func (c *Counter) Inc(d int) { c.n += d }

func bump(c *Counter, d int) { c.n += d * 3 }

type Shape interface{ Area() int }
type Rect struct{ w, h int }
type Sq struct{ s int }

func (r *Rect) Area() int { return r.w * r.h }
func (s *Sq) Area() int   { return s.s * s.s }

func stackObjects(n int) int {
	total := 0
	for i := 0; i < n; i++ {
		var c Counter
		c.Inc(i)
		bump(&c, 2)
		p := &Counter{n: i}
		p.Inc(1)
		bump(p, 1)
		total += c.n + p.n
	}
	return total
}

func localClosure(n int) int {
	sum := 0
	add := func(k int) { sum += k }
	for i := 0; i < n; i++ {
		add(i & 7)
		func() { sum ^= i }()
	}
	return sum
}

func elements(shapes []Shape, rounds int) int {
	total := 0
	for r := 0; r < rounds; r++ {
		for _, s := range shapes {
			total += s.Area()
		}
		for i := range shapes {
			v := shapes[i]
			total += v.Area() & 3
		}
	}
	return total
}

func main() {
	n := benchN
	shapes := []Shape{&Rect{2, 3}, &Sq{4}, &Rect{5, 6}, &Sq{7}}
	fmt.Println(stackObjects(n), localClosure(4*n), elements(shapes, n/2))
}
