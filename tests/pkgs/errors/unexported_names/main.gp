// phase: 06

// Only the exported names of another package can be used; an unexported one is an error at
// the use, whatever C++ would allow (the header holds all the declarations).
package main

import "example.com/e2/geom"

func main() {
	_ = geom.hidden{} // ERROR "name hidden not exported by package geom"
	_ = geom.f()      // ERROR "name f not exported by package geom"
	_ = geom.Pt{x: 1} // ERROR "cannot refer to unexported field x in struct literal of type geom.Pt"
	_ = geom.v        // ERROR "name v not exported by package geom"
	_ = geom.c        // ERROR "name c not exported by package geom"
	var p geom.Pt
	_ = p.x           // ERROR "p.x undefined \(cannot refer to unexported field x\)"
	_ = geom.Nothing  // ERROR "undefined: geom.Nothing"
}
