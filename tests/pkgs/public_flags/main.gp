// phase: 06

// Public flags (Phase 06 step 6.3): package geo has //gocpp:cflags -DGEO_SCALE=3, and a struct of its
// header has a field of a type of geo.h (a C struct whose layout depends on that macro). So pkg.hpp
// includes geo.h, and every package that includes pkg.hpp (mid, main) compiles with the flags of geo and
// with its folder for "./geo.h": geo.h says #error without the macro.
package main

import (
	"fmt"

	"example.com/hdr/geo"
	"example.com/hdr/mid"
)

func main() {
	var c geo.Cell
	geo.Fill(&c, 10)
	fmt.Println(c.N, c.Total())
	g := &mid.Grid{Name: "g"}
	geo.Fill(&g.Cells[0], 1)
	geo.Fill(&g.Cells[1], 100)
	mid.Show(g)
}

// Output:
// 33 33
// g 6 303 309
