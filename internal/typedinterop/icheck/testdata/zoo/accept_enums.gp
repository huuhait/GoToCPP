// Enums: enumerators go where the enum is wanted, an unscoped enum converts to an integer,
// a scoped one needs its enumerator.
package main

import (
	"./shapes.hpp"
	"fmt"
)

func main() {
	shapes.paint(shapes.Green)
	var c shapes.Color = shapes.Blue
	shapes.paint(c)
	shapes.setmode(shapes.Mode.Slow)
	var k int = int(shapes.Red)
	k = int(shapes.Blue)
	var m shapes.Mode = shapes.Mode.Fast
	shapes.setmode(m)
	fmt.Println(k, c == shapes.Blue, int(shapes.Mode.Slow))
}
