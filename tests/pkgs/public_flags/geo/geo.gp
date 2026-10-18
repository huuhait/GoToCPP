//gocpp:cflags -DGEO_SCALE=3
package geo

import "./geo.h"

// Cell has a field of a C type: pkg.hpp needs geo.h, and so do the packages that include pkg.hpp.
type Cell struct {
	N int
	P geo.geo_pt
}

func Fill(c *Cell, x int) {
	geo.geo_fill(&c.P, C.int(x))
	c.N = int(geo.geo_sum(&c.P))
}

func (c *Cell) Total() int { return int(geo.geo_sum(&c.P)) }

