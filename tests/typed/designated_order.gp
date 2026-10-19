// phase: 12
// opaque: reject

// A keyed literal of a C/C++ aggregate is a designated initializer in declaration order, whatever
// the order of the keys in the Go source. Go's order of evaluation (source order) holds for the
// calls in the values.
package main

import (
	"fmt"

	"./order.hpp"
)

func main() {
	r := order.Rect{h: 480, w: 640, y: 5, x: 1}
	fmt.Println(r.x, r.y, r.w, r.h)

	// Default member initializers stay for the fields that the literal does not name.
	c := order.Config{scale: 2.5, id: 42, in: order.Inner{b: 2, a: 1}, arr: [3]C.int{4, 5, 6}, mode: 5}
	fmt.Println(c.id, C.GoString(c.name), c.arr[0], c.arr[1], c.arr[2], c.in.a, c.in.b, int(c.flags), int(c.mode), c.scale)

	d := order.Der{r: 30, q: 20}
	fmt.Println(d.p, d.q, d.r)

	n := order.Num{f: 1.5}
	fmt.Println(n.f)

	a := order.Anon{v: 9, u: 8, i: 7, tag: 1}
	fmt.Println(a.tag, a.i, a.u, a.v)

	// Two members of one union cannot both be designated: that literal is written with assignments.
	u := order.Anon{f: 2.5, i: 7, tag: 2}
	fmt.Println(u.tag, u.i)

	// The calls run in source order: 1, 2, 3, 4, whatever the order of the fields.
	var trace []int
	step := func(n int) int {
		trace = append(trace, n)
		return n
	}
	s := order.Rect{h: C.int(step(1)), x: C.int(step(2)), w: C.int(step(3)), y: C.int(step(4))}
	fmt.Println(s.x, s.y, s.w, s.h, trace)

	// Exactly two calls, in the other order than the fields: the first call still runs first.
	var two []int
	note := func(n int) int {
		two = append(two, n)
		return n
	}
	w2 := order.Rect{h: C.int(note(1)), x: C.int(note(2))}
	fmt.Println(w2.x, w2.h, two)

	// Calls in the elements of an array field, and in the fields around it, out of order.
	cfg := order.Cfg{in: order.Inner{b: order.add(1, 1), a: order.mul(3, 3)}, arr: [3]C.int{order.add(1, 2), 3, order.mul(2, 2)}, id: order.add(5, 6)}
	fmt.Println(cfg.id, cfg.arr[0], cfg.arr[1], cfg.arr[2], cfg.in.a, cfg.in.b)

	// Not an aggregate: its constructor runs, then the fields are assigned.
	ct := order.Ctor{b: 2}
	fmt.Println(ct.a, ct.b)

	// A number of another type is converted: no narrowing error.
	k := 70
	var small uint8 = 200
	t := order.Rect{h: k, x: C.int(small), w: 3}
	fmt.Println(t.x, t.y, t.w, t.h)
}

// Output:
// 1 5 640 480
// 42 none 4 5 6 1 2 0 5 2.5
// 1 20 30
// 1.5
// 1 7 8 9
// 2 7
// 2 4 3 1 [1 2 3 4]
// 2 1 [1 2]
// 11 3 3 4 9 2
// 5 2
// 200 0 3 70
