// Package clib is the other package of crosspkg.gp: its exported values
// and types come from C/C++, so they have no complete types there.
package clib

import (
	"../geom.h"
	"<vector>"
)

// Origin and Values are variables initialized from C/C++.
var (
	Origin = geom.make_point(0, 0)
	Values = std.vector[int]{1, 2, 3}
	Rect   = geom.new_rect(Origin, Origin)
)

// Point and Handle are named types over C types.
type Point geom.point
type Handle geom.handle

// Pair is a Go struct with a C field.
type Pair struct {
	A, B geom.point
	N    int
}

// New returns a pointer to a C struct.
func New() *geom.struct_rect { return geom.new_rect(Origin, Origin) }

// Grid returns a nested C++ container.
func Grid() std.vector[std.vector[int]] {
	return std.vector[std.vector[int]]{{1}, {2, 3}}
}

// Open returns a Handle.
func Open(name string) Handle { return Handle(geom.open_handle(C.cstr(name))) }
