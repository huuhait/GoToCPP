// phase: 06

// Packages of another module: a required module that a replace line points at a folder, with a
// package of its own below it. Its packages are packages like those of the main module: headers,
// units and init order alike.
package main

import (
	"fmt"

	"example.com/shapes"
)

func main() {
	fmt.Println(shapes.Square{4}.Area())
}

// Output:
// init inner
// init shapes
// 16
