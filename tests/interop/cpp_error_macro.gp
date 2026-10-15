// phase: 04

// An error inside a function-like macro of a header is reported at the .gp line that uses the
// macro, not at the header import (gcc puts the error in the header, with a note "in expansion
// of macro" at the use).
package main

import "./errmacro.h"

func main() {
	n := 3
	_ = errmacro.POINT_X(C.int(n)) // ERROR "C\+\+: (base operand of '->' is not a pointer|member reference type 'int' is not a pointer)"
}
