// phase: 04

// Errors in literals of C/C++ types that only the C++ compiler sees (step 4.6), at their .gp
// lines: an array field set by a keyed literal (gp::cgo::field_ok's static_assert: an array can't be
// assigned, until Phase 12), a narrowing conversion in a braced list (a gocpp int into a C int
// field, as in cgo: both compilers reject it), more values than fields, and Go's index keys
// for a C++ sequence (a key:value literal of a C++ type must be a map's).
package main

import (
	"<array>"
	"<vector>"

	"./lits.h"
)

func main() {
	n := 3
	_ = lits.Triple{v: {1, 2, 3}, n: 3} // ERROR "C\+\+: a keyed literal of a C/C\+\+ type can't set an array field \(until Phase 12\): write a positional literal, T\{a, b, c\}"
	_ = lits.Vec{1, n}                  // ERROR "C\+\+: .*narrow"
	_ = lits.Vec{y: n}                  // ERROR "C\+\+: .*narrow"
	_ = lits.Vec{1, 2, 3}               // ERROR "C\+\+: (too many initializers|excess elements in struct initializer)"
	_ = std.vector[int]{2: 7}           // ERROR "C\+\+: a keyed literal of a C\+\+ type with keys that aren't field names needs a map \(std.map, std.unordered_map, ...\)"
	_ = std.array[C.int, 3]{2: 7}       // ERROR "C\+\+: a keyed literal of a C\+\+ type with keys that aren't field names needs a map"
	_ = lits.Triple{{1, 2, 3}, C.int(n)}
}
