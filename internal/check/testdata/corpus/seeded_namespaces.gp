// Seeded mistakes: typos in namespace names (also of a local header's
// namespace, and in a namespace rename), next to correct uses.
package main

import (
	"fmt"

	"./shapes.hpp"
	"<filesystem>"
	"<vector>"
	fsys "::std::filesystm" // ERROR "is not a namespace of the imported headers"
)

func main() {
	v := stdd.vector[int]{1} // ERROR "undefined: stdd"
	w := std.vector[int]{2}
	g := geoo.Vec{} // ERROR "undefined: geoo"
	n := geo.detail.scale
	fmt.Println(v, w, g, n, int(w.size()))
	p := fsys.path("/")
	fmt.Println(p)
	fmtt.Println("x")         // ERROR "undefined: fmtt"
	var d stdd.chrono.seconds // ERROR "undefined: stdd"
	_ = d
}
