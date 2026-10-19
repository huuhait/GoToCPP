// A method value is an error, and the calls of the same method are not.
package main

import "./shapes.hpp"

func main() {
	var c shapes.Counter = shapes.make_counter()
	c.inc()
	f := func() { c.inc() }
	f()
	m := c.inc; _ = m
	for c.get() < 5 {
		c.inc()
	}
}
