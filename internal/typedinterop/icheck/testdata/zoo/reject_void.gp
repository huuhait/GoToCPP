// A void C++ call used as a value.
package main

import (
	"./shapes.hpp"
	"<cstdio>"
)

func use(n int) {}

func main() {
	x := shapes.nothing(); _ = x
	var y int = shapes.nothing(); _ = y
	use(shapes.nothing())
	cstdio.puts(shapes.nothing())
}
