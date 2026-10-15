// phase: 04

// A header package whose name is also a namespace (geo.h declares namespace geo): geo.X is
// looked up in the namespace first (::geo::vec3), then among the header's C-level names
// (::geo_version); a cgo tag name (geo.struct_geo_pair) is struct geo_pair.
package main

import (
	"fmt"

	"./geo.h"
)

func length2(v geo.vec3) C.double { return geo.len2(v) }

func main() {
	var v geo.vec3 = geo.make(1, 2, 2)
	var p geo.struct_geo_pair = geo.geo_make_pair(3, 4)
	fmt.Println(float64(length2(v)), int(geo.geo_version()), int(geo.geo_sum(p)))
}

// Output:
// 9 3 7
