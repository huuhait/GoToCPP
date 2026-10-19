// Members that do not exist, are private, or are methods used as static functions.
package main

import "./shapes.hpp"

func main() {
	var p shapes.Point
	var sec shapes.Secret
	_, _ = p, sec
	_ = p.nosuch
	p.nosuch2()
	_ = p.Sum()
	_ = sec.hidden
	_ = shapes.Point.sum(1)
}
