// Calls of names that are not functions, and names that are not there.
package main

import (
	"./shapes.hpp"
	"<cstdio>"
	"<chrono>"
)

func main() {
	shapes.nosuchfn(1)
	cstdio.printff("x")
	_ = std.chrono.steady_clok.now()
}
