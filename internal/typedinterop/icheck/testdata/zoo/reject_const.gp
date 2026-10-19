// const-correctness: a method that changes the object cannot be called on a const one.
package main

import "./shapes.hpp"

func main() {
	shapes.gcounter().inc()
	shapes.cpoint().move(1)
}
