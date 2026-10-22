// phase: 04

// 3d.h would be named 3d, which is no Go name: the name d3 is undefined, and gocpp adds how to
// name the header.
package main

import "./3d.h"

func main() {
	println(d3.d3_mn()) // ERROR "undefined: d3; \x22./3d.h\x22 has no package name, because 3d is not a valid Go name: give it a name: import d3 \x22./3d.h\x22"
}
