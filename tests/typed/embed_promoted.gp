// phase: 12
// opaque: reject

// The members of an embedded C++ class are promoted into the Go struct, with Go's rules of selectors:
// fields, methods with const and non-const overloads, members of a base class, static members,
// enumerators, member templates (deduced), `T&` results that stay assignable, through embedded
// pointers and embedded structs, and two classes in one struct when only one has the name. Typed
// mode asks the C++ compiler which class has which name; opaque mode refuses several classes.
package main

import (
	"fmt"

	"<vector>"
	"./embed_promoted.hpp"
)

type Widget struct {
	embed_promoted.Colored
	label string
}

type Panel struct {
	*Widget
	embed_promoted.Tag
	id int
}

type Outer struct {
	Panel
	n int
}

type Stacked struct {
	embed_promoted.Stack
	std.vector[int]
}

func main() {
	w := Widget{label: "w"}
	w.grow(2) // Colored's base Shape
	w.rgb = 9
	w.w++
	fmt.Println(int(w.area()), int(w.color()), int(w.w), w.label)

	p := Panel{Widget: &Widget{}, id: 1}
	p.grow(1) // through the embedded pointer
	p.set("hello")
	p.h += 5
	fmt.Println(int(p.area()), int(p.length()), p.text, p.id)

	o := Outer{Panel: p, n: 3}
	o.grow(1) // through a struct and a pointer
	o.set("deep")
	fmt.Println(int(o.area()), int(o.length()), int(o.rgb), o.n, int(p.area()))

	var s Stacked
	s.push(4)
	s.push(5)
	s.push_back(6) // the vector's, not the stack's
	s.push_back(7)
	t := s.top() // the non-const overload counts the call
	s.top() = 8  // and returns a T&
	q := &s.top()
	*q += 1
	top := int(s.top())
	calls := int(s.top_calls)
	fmt.Println(int(t), top, calls, int(s.size()), int(s.items.size()))
	b1 := int(s.bump())
	b2 := int(s.bump())
	fmt.Println(int(s.Fast), int(s.Slow), b1, b2, int(s.made))
	last := int(s.Stack.top())
	fmt.Println(int(s.twice(21)), s.twice(1.5), int(s.at(1)), last, int(s.vector.size()))
}

// Output:
// 12 9 4 w
// 14 5 hello 1
// 24 4 7 3 24
// 5 9 4 2 2
// 3 4 1 2 2
// 42 3 7 9 2
