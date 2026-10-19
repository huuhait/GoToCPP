// A C++ reference parameter needs a variable, not a temporary.
package main

import "./shapes.hpp"

func main() {
	shapes.scale(shapes.make_point(), 2)
	shapes.take_nocopy_ref(shapes.make_nocopy())
}
