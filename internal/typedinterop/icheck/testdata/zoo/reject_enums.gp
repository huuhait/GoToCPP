// Enums: an integer is not an enumerator, and one enum is not another.
package main

import "./shapes.hpp"

func main() {
	shapes.paint(2)
	shapes.setmode(0)
	shapes.paint(shapes.Mode.Fast)
	shapes.setmode(shapes.Blue)
	shapes.paint(shapes.Blue)
}
