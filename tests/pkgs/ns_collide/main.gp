// phase: 06

// Import paths that differ only in one separator character: a_b, a.b, a-b, a~b, a+b and a/b. Each
// gets its own namespace (the escapes of NamespaceName never give the same text), so their types
// and variables don't clash, the interface values of their types are all different, and their
// namespaces are not the same C++ names.
package main

import (
	"fmt"

	under "example.com/nscollide/k/a_b"
	dot "example.com/nscollide/k/a.b"
	dash "example.com/nscollide/k/a-b"
	tilde "example.com/nscollide/k/a~b"
	plus "example.com/nscollide/k/a+b"
	slash "example.com/nscollide/k/a/b"
)

func main() {
	all := []any{under.Make(), dot.Make(), dash.Make(), tilde.Make(), plus.Make(), slash.Make()}
	equal := 0
	for i, x := range all {
		for j, y := range all {
			if i != j && x == y {
				equal++
			}
		}
	}
	fmt.Println(equal, under.V, dot.V, dash.V, tilde.V, plus.V, slash.V)
	fmt.Println(under.F(), dot.F(), dash.F(), tilde.F(), plus.F(), slash.F())
}

// Output:
// 0 under dot dash tilde plus slash
// 5 3 4 5 4 5
