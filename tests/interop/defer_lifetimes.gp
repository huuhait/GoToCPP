// phase: 04

// A local dies when its C++ block ends, which is before the function's deferred calls run: so a
// deferred call keeps alive every C++ local that its object or its arguments mention, not only the
// root of the object: defer v.front().rename(...) (a reference into v), defer lib.f(v.data())
// (a pointer into v), and the local that a pointer holds the address of (p := &a, then
// defer p.rename(...)). Each of these was a use after free (found by ASan in review).
package main

import (
	"fmt"

	"./objects.h"
	"<string>"
	"<vector>"
)

func front() {
	v := std.vector[objects.Guard]{}
	v.emplace_back(std.string("a"))
	defer v.front().rename(std.string("front"))
	defer v.at(0).note("at")
	fmt.Println("front body")
}

func data() {
	v := std.vector[objects.Guard]{}
	v.emplace_back(std.string("d"))
	defer objects.rename_ptr(v.data(), "data")
	fmt.Println("data body")
}

func address() {
	a := objects.Guard(std.string("p"))
	p := &a
	defer p.rename(std.string("p-renamed"))
	q := p // the address travels through q
	defer objects.rename_ptr(q, "q-renamed")
	fmt.Println("address body")
}

type Node struct {
	g *objects.Guard
}

// The address of a local that a gocpp container or struct keeps: the frame holds the box of every
// local whose address is taken in a function with defers, so no pointer to it dangles when the
// deferred calls run, wherever the pointer was stored.
func container() {
	a := objects.Guard(std.string("slice"))
	ps := []*objects.Guard{&a}
	defer objects.rename_ptr(ps[0], "slice-renamed")
	b := objects.Guard(std.string("append"))
	var qs []*objects.Guard
	qs = append(qs, &b)
	defer objects.rename_ptr(qs[0], "append-renamed")
	c := objects.Guard(std.string("map"))
	m := map[string]*objects.Guard{"c": &c}
	defer objects.rename_ptr(m["c"], "map-renamed")
	d := objects.Guard(std.string("struct"))
	n := &Node{g: &d}
	defer n.g.rename(std.string("struct-renamed"))
	fmt.Println("container body")
}

func main() {
	front()
	data()
	address()
	container()
	fmt.Println("done")
}

// Output:
// lock a
// front body
// a: at
// unlock front
// lock d
// data body
// unlock data
// lock p
// address body
// unlock p-renamed
// lock slice
// lock append
// lock map
// lock struct
// container body
// unlock slice-renamed
// unlock append-renamed
// unlock map-renamed
// unlock struct-renamed
// done
