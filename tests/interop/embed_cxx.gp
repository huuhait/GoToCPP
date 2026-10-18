// phase: 05

// A struct that embeds a C/C++ type gets its fields and methods promoted (the C++ compiler checks
// the member): o.x is o.epoint.x, also through embedded structs and embedded pointers, for reads,
// writes, ++ and method calls; the embedded value is a copy with the struct, and a Go method of
// the outer type hides the C++ member of the same name only for the Go type's own name.
package main

import (
	"fmt"

	"./embed_cxx.h"
	"<vector>"
)

type P struct {
	embed_cxx.epoint
	name string
}

type Labeled struct {
	embed_cxx.elabel
	n int
}

type Vec struct {
	std.vector[int]
	tag int
}

type Outer struct {
	*Vec
	n int
}

func (p P) Name() string { return p.name }

func main() {
	p := P{name: "p"}
	p.x = 3
	p.y = 4
	p.y += 2
	p.x++
	fmt.Println(int(p.x), int(p.y), int(p.sum()), p.Name())
	p.scale(2)
	fmt.Println(int(p.epoint.x), int(p.epoint.y), int(p.sum()))
	q := p // a copy
	q.scale(10)
	fmt.Println(int(p.x), int(q.x))
	pp := &q
	pp.x = 1
	pp.scale(3)
	fmt.Println(int(pp.sum()), int(q.sum()))

	var v Vec
	v.push_back(1)
	v.push_back(2)
	v.push_back(3)
	fmt.Println(int(v.size()), v.tag, bool(v.empty()))
	o := Outer{&Vec{}, 1}
	o.push_back(7)
	o.push_back(8)
	fmt.Println(int(o.size()), int(o.Vec.vector.size()), o.n)
	for i := 0; i < int(o.size()); i++ {
		fmt.Print(" ", int(o.at(i)))
	}
	fmt.Println()
	o.pop_back()
	fmt.Println(int(o.size()), int(o.back()))

	var l Labeled
	l.append("hello")
	l.append("!")
	l.n = 5
	fmt.Println(int(l.size()), l.n, l.text == "hello!")
}

// Output:
// 4 6 10 p
// 8 12 20
// 8 80
// 363 363
// 3 0 false
// 2 2 1
//  7 8
// 1 7
// 6 5 true
