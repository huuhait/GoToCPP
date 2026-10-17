// phase: 04

// The results of a function with defers go through variables (the single-exit shape). For C++
// types that is a gp::result_slot (step 4.8), which stores the result where C++ allows: a class
// with no default constructor, a move-only type, several such results, and a named result that a
// deferred closure changes. A local that is returned is moved, not copied.
package main

import (
	"fmt"

	"./objects.h"
	"<memory>"
	"<utility>"
	"<vector>"
)

// Widget has no default constructor: its result variable is built by the return statement.
func makeWidget(n int) objects.Widget {
	defer fmt.Println("makeWidget", n)
	if n > 5 {
		return objects.Widget(100)
	}
	w := objects.Widget(C.int(n))
	return w
}

func makeUnique(n int) std.unique_ptr[C.int] {
	defer fmt.Println("makeUnique", n)
	p := std.make_unique[C.int](C.int(n))
	return p // moved
}

func pair(n int) (objects.Widget, std.unique_ptr[C.int]) {
	defer fmt.Println("pair", n)
	p := std.make_unique[C.int](C.int(n * 2))
	return objects.Widget(C.int(n)), p
}

// A named result: the variable exists from the start (zero value), a deferred closure can change
// it after the return statement set it.
func named(n int) (v std.vector[int]) {
	defer func() { v.push_back(99) }()
	v.push_back(C.int(n))
	if n > 1 {
		return std.vector[int]{7, 8}
	}
	return
}

func guarded() (h objects.Handle) {
	defer func() {
		if h.get() >= 0 {
			fmt.Println("closing in defer", int(h.get()))
		}
	}()
	return objects.make_handle(5) // moved into the result
}

func main() {
	w := makeWidget(3)
	fmt.Println(int(w.get()), int(makeWidget(9).get()))
	p := makeUnique(8)
	fmt.Println(int(*p))
	a, b := pair(4)
	fmt.Println(int(a.get()), int(*b))
	v := named(1)
	fmt.Println(int(v.size()), int(v[0]), int(v[1]))
	v = named(2)
	fmt.Println(int(v.size()), int(v[0]), int(v[1]), int(v[2]))
	h := guarded()
	fmt.Println("got", int(h.get()))
}

// Output:
// makeWidget 3
// makeWidget 9
// 3 100
// makeUnique 8
// 8
// pair 4
// 4 8
// 2 1 99
// 3 7 8 99
// closing in defer 5
// got 5
// close 5
