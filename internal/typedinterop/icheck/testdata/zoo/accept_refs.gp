// References: variables and fields bind to T& parameters, T& results are assignable,
// a const reference result calls const methods, a pointer or a value gives the same members.
package main

import (
	"./shapes.hpp"
	"<algorithm>"
	"<string>"
	"<utility>"
	"<vector>"
	"fmt"
)

func main() {
	var v std.vector[int]
	v.push_back(1)
	v.push_back(2)
	v[0] = 5
	var pt shapes.Point
	pt.x = 3
	shapes.scale(pt, 2)
	pp := &pt
	shapes.scale(*pp, 2)
	var a, b int = 1, 2
	std.swap(a, b)
	var c shapes.Counter = shapes.make_counter()
	c.inc()
	fmt.Println(shapes.gcounter().get(), shapes.cpoint().sum(), c.get(), pt.sum(), pp.sum(), v.front(), v.back(), a, b)
	var nc shapes.NoCopy
	shapes.take_nocopy_ref(nc)
	var s std.string = std.string("hello")
	s.push_back('!')
	fmt.Println(s.size(), shapes.takes_str(s))
}
