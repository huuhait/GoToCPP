// Seeded mistake: a misspelled local header, next to a correct one.
package main

import (
	"fmt"

	"./geom.h"
	"./geomm.h" // ERROR "cannot find header \x22./geomm.h\x22"
)

func main() {
	p := geom.make_point(1, 2)
	fmt.Println(float64(p.x))
}
