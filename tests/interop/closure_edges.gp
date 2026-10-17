// phase: 04

// More forms of boxed C++ objects (step 4.8): assignment and swap of a captured object, several
// values from one C++ call, declarations in if and switch headers, a named result without defers,
// a shadowing declaration, a receiver-like use through a pointer, and the object in a loop that
// breaks and continues.
package main

import (
	"fmt"

	"./objects.h"
	"<algorithm>"
	"<utility>"
	"<vector>"
)

func named() (v std.vector[int], n C.int) {
	grow := func() { v.push_back(1); n++ }
	grow()
	grow()
	return
}

func assign() {
	v := std.vector[int]{1}
	show := func() int { return int(v.size()) }
	v = std.vector[int]{1, 2, 3}
	fmt.Println(show())
	w := std.vector[int]{9}
	v, w = w, v // a swap through temporaries
	fmt.Println(show(), int(w.size()))
	v = w
	w.push_back(4)
	fmt.Println(show(), int(w.size()))
}

func pairs() {
	lo, hi := std.minmax(3, 1)
	show := func() { fmt.Println(int(lo), int(hi)) }
	lo = 7
	show()
}

func headers() {
	if c := (objects.Counter{}); c.get() == 0 {
		inc := func() { c.inc() }
		inc()
		inc()
		fmt.Println("if", int(c.get()))
	}
	switch c := (objects.Counter{}); c.get() {
	case 0:
		inc := func() { c.add(3) }
		inc()
		fmt.Println("switch", int(c.get()))
	}
}

func shadow() {
	v := std.vector[int]{1}
	f := func() { v.push_back(2) }
	{
		v := std.vector[int]{} // another variable
		g := func() { v.push_back(3); v.push_back(4) }
		g()
		fmt.Println("inner", int(v.size()))
	}
	f()
	fmt.Println("outer", int(v.size()))
}

func loops() {
	var c objects.Counter
	add := func(d int) { c.add(C.int(d)) }
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			continue
		}
		if i > 7 {
			break
		}
		add(i)
	}
	fmt.Println(int(c.get()))
	var w objects.Widget = objects.Widget(1)
	i := 0
	for ; i < 3; i++ {
		func() { w.set(w.get() * 2) }()
	}
	fmt.Println(int(w.get()), i)
}

func main() {
	v, n := named()
	fmt.Println(int(v.size()), int(n))
	assign()
	pairs()
	headers()
	shadow()
	loops()
}

// Output:
// 2 2
// 3
// 1 3
// 3 4
// 7 3
// if 2
// switch 3
// inner 2
// outer 2
// 16
// 8 3
