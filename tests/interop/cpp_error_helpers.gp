// phase: 04

// Errors inside gocpp's interop helpers (gocpp/interop.hpp), which a .gp line instantiated, are
// reported at that line. A static_assert of the helpers shows its own message, which is written
// for users.
package main

import "./tags.h"

func main() {
	p := tags.make_point(1, 2)
	switch p {
	case p: // ERROR "C\+\+: a switch on a C/C\+\+ value needs a number, an enum or a pointer"
	}
	n := C.int(p) // ERROR "C\+\+: (invalid 'static_cast' from type 'point' to type 'int'|cannot convert 'point' to 'int')"
	_ = n
}
