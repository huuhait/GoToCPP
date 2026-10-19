// Operators C++ does not define for these types.
package main

import (
	"./shapes.hpp"
	"<string>"
)

func main() {
	var p, q shapes.Point
	var s std.string
	_, _, _ = p, q, s
	_ = p + 1
	_ = -p
	_ = p == q
	_ = s + 1
	_ = !p
}
