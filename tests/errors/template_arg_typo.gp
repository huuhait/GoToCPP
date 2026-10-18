// phase: 01

// Typos inside the template arguments of opaque types are reported: Check
// type-checks the arguments that go/types skips (Phase 01 README §2.7).
package main

import (
	"strings" // ERROR "package strings is not available yet \(Phase 09\)"

	"<map>"
	"<string>"
	"<vector>"
)

type Point struct{ X int }

func main() {
	_ = std.vector[Pointt]{}                  // ERROR "undefined: Pointt"
	_ = std.map[std.string, strings.Buildr]{} // ERROR "undefined: strings.Buildr"
	_ = std.vector[Point]{}
	_ = std.vector[std.vector[Undef]]{} // ERROR "undefined: Undef"
}
