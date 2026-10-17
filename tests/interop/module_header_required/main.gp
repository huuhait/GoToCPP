// phase: 04

// A header of a required module (Phase 06 step 6.1b): the module is replaced by a local folder here, and
// the longest module path that is a prefix of the import path names the module. mathlib.hpp includes
// "detail/consts.hpp" by its name in the module. The project folder has a mathlib.hpp of its own (an #error):
// it is not the module's header, so it never gets included. backends/gl.hpp includes "mathbase.hpp", a
// header at the root of its module, which works because the module's folder is on the include path (as
// -iquote: a string.h at the root of a module, here an #error, doesn't hide <cstring> or <string.h>).
package main

import (
	"fmt"

	"example.com/game/shapes.hpp"
	"example.org/mathlib/backends/gl.hpp"
	"example.org/mathlib/mathlib.hpp"
)

func main() {
	fmt.Println(mathlib.ml_square(7), mathlib.ml_scaled(2))
	fmt.Println(shapes.sh_sides(), gl.ml_cube(3))
}

// Output:
// 49 6
// 4 28
