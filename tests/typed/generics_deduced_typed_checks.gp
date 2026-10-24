// phase: 12
// opaque: reject

// A variable declared from a generic call whose type arguments C++ deduces (k2 := maps.Clone(k), with
// k a map[int]std.string) is a gocpp map to the build (lower/deducedres.go), and the package's other C/C++
// values keep typed mode's checks: s.empty() returns a C++ bool, which a Go bool takes (it was refused,
// "cannot use s.empty() (C/C++ value) as bool", when the lowering made the package's type maps its own:
// typed interop is off for such a package).
package main

import (
	"fmt"
	"maps"

	"<string>"
)

func isE(s std.string) bool { return s.empty() }

func main() {
	k := map[int]std.string{1: "a"}
	k2 := maps.Clone(k)
	var ok bool = k[1].empty()
	n := 3 + k[1].size()
	fmt.Println(len(k2), isE(k[1]), ok, int(n))
}

// Output:
// 1 false false 4
