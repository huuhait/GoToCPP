// Wrong arguments to C and C++ functions: an integer for a char*, a Go string for a
// char*, a class for a pointer to it, too many and too few arguments.
package main

import (
	"./shapes.hpp"
	"<cstdio>"
	"<cstdlib>"
	"<cstring>"
)

func main() {
	var s string = "hello"
	var p shapes.Point
	_, _ = s, p
	_ = cstring.strlen(42)
	cstdio.puts(s)
	_ = shapes.count(p)
	_ = cstdlib.abs(1, 2)
	_ = cstdlib.abs()
	_ = shapes.defaults(1, 2, 3)
}
