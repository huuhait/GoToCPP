// phase: 04

// A replace to a plain C++ repository (Phase 06 step 6.1b): deps/lib is a clone of a library with no
// gocpp.mod file, and that is fine, it is the module github.com/vendor/geometry of the replace line. The
// header geometry.hpp includes "include/lib/area.hpp", a name inside the repository.
package main

import (
	"fmt"

	"github.com/vendor/geometry/geometry.hpp"
)

func main() {
	fmt.Println(geometry.geo_area(3, 4))
}

// Output:
// 12
