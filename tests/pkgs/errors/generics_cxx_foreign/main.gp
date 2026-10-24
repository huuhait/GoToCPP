// phase: 07

// A generic function of another package whose body uses C/C++ names is instantiated in the package
// that uses it, with that package's C/C++ state, which gocpp does not do yet: check says so at each
// use, as the build does (also in a package without C/C++ names of its own); a generic function
// without them is fine, and a use inside a generic declaration is reported where it is instantiated.
package main

import (
	"fmt"

	"example.com/gcf/lib"
)

func Inner[T any](v T) int { return lib.Size(v) }

func main() {
	f := lib.Ret(4)            // ERROR "lib.Ret uses C/C\+\+ names, so it can only be instantiated in its own package \(Phase 12\)"
	n := lib.Size[string]("x") // ERROR "lib.Size uses C/C\+\+ names, so it can only be instantiated in its own package \(Phase 12\)"
	g := lib.Size[int]         // ERROR "lib.Size uses C/C\+\+ names"
	fmt.Println(f != nil, n, g != nil, lib.Plain(3))
}
