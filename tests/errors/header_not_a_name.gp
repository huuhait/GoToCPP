// phase: 01

// headers/3d.h would be named 3d, which is no Go name. The name d3 is
// undefined, and the hint says how to import the header with it.
package main

import "./headers/3d.h"

func main() {
	println(d3.d3_mn()) // ERROR "undefined: d3; \x22./headers/3d.h\x22 has no package name, because 3d is not a valid Go name: give it a name: import d3 \x22./headers/3d.h\x22"
}
