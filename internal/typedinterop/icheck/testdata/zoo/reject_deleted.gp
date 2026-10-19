// A class that cannot be copied is passed by value.
package main

import "./shapes.hpp"

func main() {
	var nc shapes.NoCopy
	_ = nc
	shapes.take_nocopy(nc)
}
