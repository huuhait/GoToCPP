// phase: 01

// The same header imported twice is a redeclared name, as in Go.
package main

import (
	"<cmath>"
	"<cmath>" // ERROR "cmath redeclared in this block"
)

func main() {}
