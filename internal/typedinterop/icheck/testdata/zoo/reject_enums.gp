// Enums: an integer variable is not an enumerator, and one enum is not another. (An integer
// constant is adapted by the lowering: static_cast, ADR-130, so the checker does not report it.)
package main

import "./shapes.hpp"

func main() {
	n := 2
	shapes.paint(n)
	shapes.setmode(n)
	shapes.paint(2)
	shapes.setmode(0)
	shapes.paint(shapes.Mode.Fast)
	shapes.setmode(shapes.Blue)
	shapes.paint(shapes.Blue)
}
