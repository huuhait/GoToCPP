// Conversions of opaque values keep their Go types, so mistakes after
// them are still found (here: none).
package main

import (
	"fmt"
	"strconv"

	"./geom.h"
	"<cmath>"
	"<cstring>"
	"<vector>"
)

type Meters float64

func length(v std.vector[int]) int { return int(v.size()) }

func main() {
	v := std.vector[int]{1, 2, 3}
	n := int(v.size())
	fmt.Println(n+1, strconv.Itoa(n), length(v)*2)
	root := float64(std.sqrt(2.0))
	m := Meters(std.hypot(3, 4))
	fmt.Println(root*2, m+1, float32(std.floor(root)) > 0)
	size := uint64(std.strlen(C.cstr("abc")))
	idx := int(v[0]) + int(v.at(1))
	fmt.Println(size, idx, int64(len("x"))+int64(v.back()))
	p := geom.make_point(1, 2)
	d := float64(geom.point_dist(p, p)) + float64(p.x)
	fmt.Println(d, C.int(n), C.double(d))
	bytes := []byte(C.GoString(C.cstr("hi")))
	fmt.Println(len(bytes), string(bytes))
	x := v.front()
	k := int(x)
	k += 2
	fmt.Println(k, int(geom.GEOM_MAX_POINTS)/2, uint8(geom.GREEN))
}
