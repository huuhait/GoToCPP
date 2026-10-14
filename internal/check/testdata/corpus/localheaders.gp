// Local headers: a C header (structs, a typedef, an enum, a macro) and a
// C++ header with namespaces; named types over the C structs, with
// methods; values of C types in Go types.
package main

import (
	"fmt"

	"./geom.h"
	"./shapes.hpp"
)

// Named types over C types.
type Point geom.point
type Handle geom.handle
type Rect geom.struct_rect

func (p Point) Dist(q Point) float64 {
	dx, dy := p.x-q.x, p.y-q.y
	return float64(dx*dx + dy*dy)
}

func (p *Point) Move(dx float64) {
	p.x += dx
}

type Scene struct {
	origin  Point
	shapes  []geo.Vec
	handles map[Handle]string
	rect    *geom.struct_rect
}

const maxHandles Handle = 8

func newScene() *Scene {
	s := &Scene{origin: Point{x: 1, y: 2}, handles: map[Handle]string{}}
	s.shapes = append(s.shapes, geo.Vec{x: 3, y: 4})
	s.rect = geom.new_rect(geom.make_point(0, 0), geom.make_point(1, 1))
	return s
}

func main() {
	s := newScene()
	defer geom.free_rect(s.rect)
	p := Point{x: 3, y: 4}
	p.Move(1)
	fmt.Println(p.Dist(s.origin), p.x)
	var h Handle = Handle(geom.open_handle(C.cstr("a")))
	s.handles[h] = "a"
	s.handles[maxHandles] = "max"
	if h == 0 || h+1 > maxHandles {
		fmt.Println("no handle")
	}
	h++
	hs := []Handle{h, maxHandles}
	hs = append(hs, 1, 2)
	slots := make([]string, maxHandles)
	slots[h] = "first"
	fmt.Println(len(hs), slots[h:], min(h, maxHandles))
	pts := []Point{{x: 1}, {x: 2, y: 3}}
	pts[0].y = float64(len(pts))
	var r Rect
	r.min = geom.make_point(0, 0)
	fmt.Println(r.max.x, geom.point_dist(r.min, r.max), geom.GEOM_MAX_POINTS)
	c := geom.RED
	if c == geom.BLUE {
		fmt.Println("blue")
	}
	var v geo.Vec = s.shapes[0]
	fmt.Println(v.len(), geo.dot(v, v), int(geo.corners(2, 3).size()))
	fmt.Println(geo.detail.scale, geo.detail.name(v).c_str(), int(geom.handle_count()))
}
