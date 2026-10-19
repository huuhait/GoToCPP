// Conversions T(x) that C++ does not allow.
package main

import (
	"./shapes.hpp"
	"<vector>"
)

func main() {
	var v std.vector[int]
	var p shapes.Point
	_, _ = v, p
	_ = float64(v)
	_ = int(p)
}
