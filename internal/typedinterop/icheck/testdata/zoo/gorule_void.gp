// Where Go's own typing rules reject what opaque mode builds and runs: a void C++ call
// is no value in Go, so it cannot be assigned to the blank identifier or be the tag of a
// switch, as an argument or in a declaration. The checker gives the same error for every
// context (ADR-079), also for a member call. A test pins the decision (TestGoRuleZoo).
package main

import "./shapes.hpp"

func main() {
	var c shapes.Counter = shapes.make_counter()
	_ = c
	_ = shapes.nothing()
	switch shapes.nothing() {
	}
	_ = c.inc()
	c.inc()
}
