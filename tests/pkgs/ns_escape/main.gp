// phase: 06

// Import paths whose escapes could collide: a/b_c and a_b/c, a.b and a_b2, x-y/z, upper case, a version-like element, a leading underscore, + and ~: each package has its own namespace and its own types (the interface values of the types of different packages are never equal).

package main

import (
	"fmt"

	bc1 "example.com/nsescape/a/b_c"
	bc2 "example.com/nsescape/a_b/c"
	ab1 "example.com/nsescape/a.b"
	ab2 "example.com/nsescape/a_b2"
	xyz "example.com/nsescape/x-y/z"
	upcase "example.com/nsescape/Up/Case"
	dotv2 "example.com/nsescape/dotted.pkg/v2"
	ux "example.com/nsescape/under_score/_x"
	plus "example.com/nsescape/plus+/x"
	tilde "example.com/nsescape/tilde~/t"
)

func main() {
	fmt.Println(bc1.Name(), bc2.Name(), ab1.Name(), ab2.Name(), xyz.Name(), upcase.Name(), dotv2.Name(), ux.Name())
	fmt.Println(bc1.V, bc2.V, ab1.V, ab2.V, xyz.K, upcase.K, dotv2.K, ux.K)
	all := []any{bc1.Make(), bc2.Make(), ab1.Make(), ab2.Make(), xyz.Make(), upcase.Make(), dotv2.Make(), ux.Make()}
	for i, x := range all {
		for j, y := range all {
			if x == y {
				fmt.Print(" ", i, "=", j)
			}
		}
	}
	fmt.Println()
	fmt.Println(plus.N(), tilde.N())
	fmt.Println(bc1.T{N: 2}.Show(), bc2.T{N: 3}.Show(), ab1.T{N: 4}.Show(), ab2.T{N: 5}.Show())
}

// Output:
// bc1 bc2 ab1 ab2 xyz upcase dotv2 ux
// bc1 bc2 ab1 ab2 k-xyz k-upcase k-dotv2 k-ux
//  0=0 1=1 2=2 3=3 4=4 5=5 6=6 7=7
// plus tilde
// bc12 bc23 ab14 ab25
