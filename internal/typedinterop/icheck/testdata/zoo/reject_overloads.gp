// Overload sets: no overload takes the arguments, and two take them equally well.
package main

import "./shapes.hpp"

func main() {
	var p shapes.Point
	_ = p
	_ = shapes.area(1)
	_ = shapes.area(p, 1, 2)
	shapes.ovl(1.5)
	shapes.ovl2(1)
}
