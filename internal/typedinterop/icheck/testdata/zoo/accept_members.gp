// Members: fields, methods, statics, pointers and values, a value's methods through a pointer.
package main

import (
	"./shapes.hpp"
	"fmt"
)

func main() {
	var p shapes.Point
	p.x, p.y = 3, 4
	p.move(1)
	q := &p
	q.move(1)
	var sec shapes.Secret
	o := shapes.Point.origin()
	fmt.Println(p.sum(), q.sum(), sec.shown, o.sum(), shapes.Counter.make(), shapes.global_value)
	shapes.global_value = 9
	shapes.nothing()
}
