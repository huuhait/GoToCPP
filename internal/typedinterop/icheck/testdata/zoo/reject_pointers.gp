// A member of a pointer to a pointer, and a string where a std::string is wanted.
package main

import (
	"./shapes.hpp"
)

func main() {
	var p *shapes.Point
	pp := &p
	var s string = "x"
	_, _, _ = p, pp, s
	_ = pp.x
	_ = shapes.takes_str(s)
	_ = shapes.takes_cstr(s)
}
