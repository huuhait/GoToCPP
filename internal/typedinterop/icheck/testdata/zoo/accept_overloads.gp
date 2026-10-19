// Overloads are resolved by C++ with the exact types, defaults fill in, and an untyped
// constant is a C int, double or char array.
package main

import (
	"./shapes.hpp"
	"<cstdio>"
	"<cstring>"
	"fmt"
)

func main() {
	var pt shapes.Point
	var l int64 = 1
	shapes.ovl(1)
	shapes.ovl(l)
	shapes.ovl2(l)
	shapes.ovl2(uint64(2))
	fmt.Println(shapes.area(pt), shapes.area(pt, 2), shapes.defaults(3))
	cstdio.printf("%d %s %f\n", 1, "x", 2.5)
	fmt.Println(cstring.strlen("abc"), shapes.takes_cstr("lit"), shapes.takes_str("lit"), shapes.count(&pt), shapes.count(nil))
}
