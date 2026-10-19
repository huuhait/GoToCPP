// phase: 12
// opaque: reject

// The object of a deferred or spawned member call is stored in the call's frame, as a pointer when
// it is a reference into a container: typed mode leaves those to gp::deref.
package main

import (
	"fmt"

	"<memory>"
	"<string>"
	"<vector>"
	"./defer_members.hpp"
)

func run() {
	v := std.vector[defer_members.Guard]{}
	v.emplace_back()
	g := defer_members.Guard{}
	p := &g
	up := std.make_unique[defer_members.Guard]()
	defer func() { fmt.Println(v.at(0).hits, v.at(0).name, g.hits, g.name, up.hits, up.name) }()
	defer v.front().note(std.string("a"))
	defer v.at(0).note(std.string("b"))
	defer p.note(std.string("c"))
	defer up.note(std.string("d"))
	fmt.Println("body")
}

func main() {
	run()
}

// Output:
// body
// 2 ba 1 c 1 d
